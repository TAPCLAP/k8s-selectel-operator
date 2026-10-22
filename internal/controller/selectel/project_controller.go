/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package selectel

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdnv1alpha1 "github.com/tapclap/k8s-selectel-operator/api/cdn.selectel/v1alpha1"
	selectelv1alpha1 "github.com/tapclap/k8s-selectel-operator/api/selectel/v1alpha1"
	"github.com/tapclap/k8s-selectel-operator/internal/selectel/iam"
)

const (
	reasonTokenIssued              = "TokenIssued"
	reasonSecretNotFound           = "SecretNotFound"
	reasonSecretNamespaceForbidden = "SecretNamespaceForbidden"
	reasonSecretInvalid            = "SecretInvalid"
	reasonAuthFailed               = "AuthFailed"
	reasonResourcesRemain          = "ResourcesRemain"

	requeueAuth  = time.Minute
	requeueInUse = 15 * time.Second
	requeueReady = 30 * time.Minute
)

// ProjectReconciler checks Selectel credentials and keeps the Project finalizer
// while CDN resources still reference the project.
type ProjectReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	CredentialsNamespace string
	Tokens               *iam.Cache
}

// +kubebuilder:rbac:groups=selectel.tapclap.com,resources=projects,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=selectel.tapclap.com,resources=projects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=selectel.tapclap.com,resources=projects/finalizers,verbs=update
// +kubebuilder:rbac:groups=cdn.selectel.tapclap.com,resources=resources,verbs=get;list;watch
// Secret access is a Role in the operator namespace, not this ClusterRole.
// See config/rbac/credentials_role.yaml.

func (r *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var project selectelv1alpha1.Project
	if err := r.Get(ctx, req.NamespacedName, &project); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !project.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, &project)
	}

	if controllerutil.AddFinalizer(&project, selectelv1alpha1.ProjectFinalizer) {
		if err := r.Update(ctx, &project); err != nil {
			return ctrl.Result{}, err
		}
	}

	ref := project.Spec.CredentialsSecretRef
	if ref.Namespace != r.CredentialsNamespace {
		log.Info("refusing secret outside the operator namespace", "secretNamespace", ref.Namespace)
		return r.setReady(ctx, &project, metav1.ConditionFalse, reasonSecretNamespaceForbidden,
			fmt.Sprintf("credentials secret must be in namespace %s", r.CredentialsNamespace), 0)
	}

	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return r.setReady(ctx, &project, metav1.ConditionFalse, reasonSecretNotFound, "credentials secret was not found", requeueAuth)
		}
		return ctrl.Result{}, err
	}
	creds, err := iam.CredentialsFromSecret(&secret)
	if err != nil {
		return r.setReady(ctx, &project, metav1.ConditionFalse, reasonSecretInvalid, err.Error(), 0)
	}

	if _, err := r.Tokens.Token(ctx, project.Name, project.Spec.AuthURL, creds); err != nil {
		log.Error(err, "failed to issue IAM token")
		return r.setReady(ctx, &project, metav1.ConditionFalse, reasonAuthFailed, "failed to issue a project-scoped IAM token", requeueAuth)
	}
	return r.setReady(ctx, &project, metav1.ConditionTrue, reasonTokenIssued, "project-scoped IAM token issued", requeueReady)
}

func (r *ProjectReconciler) finalize(ctx context.Context, project *selectelv1alpha1.Project) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(project, selectelv1alpha1.ProjectFinalizer) {
		return ctrl.Result{}, nil
	}
	var resources cdnv1alpha1.ResourceList
	if err := r.List(ctx, &resources); err != nil {
		return ctrl.Result{}, err
	}
	for i := range resources.Items {
		if resources.Items[i].Spec.ProjectRef.Name == project.Name {
			if err := r.setCondition(ctx, project, metav1.ConditionFalse, reasonResourcesRemain, "CDN resources still reference this Project"); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: requeueInUse}, nil
		}
	}
	controllerutil.RemoveFinalizer(project, selectelv1alpha1.ProjectFinalizer)
	return ctrl.Result{}, r.Update(ctx, project)
}

func (r *ProjectReconciler) setReady(ctx context.Context, project *selectelv1alpha1.Project, status metav1.ConditionStatus, reason, message string, after time.Duration) (ctrl.Result, error) {
	if err := r.setCondition(ctx, project, status, reason, message); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: after}, nil
}

func (r *ProjectReconciler) setCondition(ctx context.Context, project *selectelv1alpha1.Project, status metav1.ConditionStatus, reason, message string) error {
	changed := setCondition(&project.Status.Conditions, project.Generation, selectelv1alpha1.ConditionReady, status, reason, message)
	if !changed {
		return nil
	}
	return r.Status().Update(ctx, project)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&selectelv1alpha1.Project{}).
		Watches(&cdnv1alpha1.Resource{}, handler.EnqueueRequestsFromMapFunc(r.resourcesToProject)).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.secretsToProject)).
		Named("selectel-project").
		Complete(r)
}

func (r *ProjectReconciler) resourcesToProject(_ context.Context, obj client.Object) []reconcile.Request {
	resource, ok := obj.(*cdnv1alpha1.Resource)
	if !ok || resource.Spec.ProjectRef.Name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: resource.Spec.ProjectRef.Name}}}
}

func (r *ProjectReconciler) secretsToProject(ctx context.Context, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok || secret.Namespace != r.CredentialsNamespace {
		return nil
	}
	var projects selectelv1alpha1.ProjectList
	if err := r.List(ctx, &projects); err != nil {
		return nil
	}
	var requests []reconcile.Request
	for i := range projects.Items {
		ref := projects.Items[i].Spec.CredentialsSecretRef
		if ref.Name == secret.Name && ref.Namespace == secret.Namespace {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: projects.Items[i].Name}})
		}
	}
	return requests
}
