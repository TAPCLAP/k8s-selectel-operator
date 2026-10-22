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

package cdnselectel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"regexp"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdnv1alpha1 "github.com/tapclap/k8s-selectel-operator/api/cdn.selectel/v1alpha1"
	selectelv1alpha1 "github.com/tapclap/k8s-selectel-operator/api/selectel/v1alpha1"
	"github.com/tapclap/k8s-selectel-operator/internal/selectel/access"
	"github.com/tapclap/k8s-selectel-operator/internal/selectel/cdn"
	"github.com/tapclap/k8s-selectel-operator/internal/selectel/iam"
)

const (
	reasonNamespaceNotAllowed      = "NamespaceNotAllowed"
	reasonCertificateNotAllowed    = "CertificateNotAllowed"
	reasonProjectNotFound          = "ProjectNotFound"
	reasonProjectNotReady          = "ProjectNotReady"
	reasonSecretNamespaceForbidden = "SecretNamespaceForbidden"
	reasonSecretNotFound           = "SecretNotFound"
	reasonSecretInvalid            = "SecretInvalid"
	reasonInvalidName              = "InvalidName"
	reasonDomainNotAllowed         = "DomainNotAllowed"
	reasonDomainPatternInvalid     = "DomainPatternInvalid"
	reasonCreateRejected           = "CreateRejected"
	reasonAdoptFailed              = "AdoptFailed"
	reasonWouldCreate              = "WouldCreate"
	reasonWouldRecreate            = "WouldRecreate"
	reasonObserving                = "Observing"
	reasonDrift                    = "Drift"
	reasonProcessing               = "Processing"
	reasonSelectelError            = "SelectelError"
	reasonSynced                   = "Synced"
	reasonRateLimited              = "RateLimited"
	reasonAuthFailed               = "AuthFailed"
	reasonAPIError                 = "APIError"

	defaultProcessingInterval = 3 * time.Minute
	processingJitter          = 30 * time.Second
	requeueReady              = 5 * time.Minute
	requeueError              = time.Minute
	requeueDenied             = time.Minute
)

var selectelName = regexp.MustCompile(`^[a-zA-Zа-яА-ЯёЁ0-9\-_ ]{1,50}$`)

// ResourceReconciler creates and updates Selectel CDN resources.
type ResourceReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             events.EventRecorder
	CredentialsNamespace string
	Tokens               *iam.Cache
	CDN                  *cdn.Client
	// ProcessingInterval is the base wait before checking a resource again while
	// Selectel applies configuration. Zero uses 3 minutes. The wait also adds a
	// random 0-30s.
	ProcessingInterval time.Duration
}

func (r *ResourceReconciler) processingDelay() time.Duration {
	base := r.ProcessingInterval
	if base <= 0 {
		base = defaultProcessingInterval
	}
	return base + time.Duration(rand.Int64N(int64(processingJitter)+1))
}

// +kubebuilder:rbac:groups=cdn.selectel.tapclap.com,resources=resources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cdn.selectel.tapclap.com,resources=resources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cdn.selectel.tapclap.com,resources=resources/finalizers,verbs=update
// +kubebuilder:rbac:groups=selectel.tapclap.com,resources=projects,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// Secret get/list/watch is granted by the operator-namespace Role in config/rbac/credentials_role.yaml.

func (r *ResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var resource cdnv1alpha1.Resource
	if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !resource.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, &resource)
	}
	if controllerutil.AddFinalizer(&resource, cdnv1alpha1.ResourceFinalizer) {
		if err := r.Update(ctx, &resource); err != nil {
			return ctrl.Result{}, err
		}
	}

	project, result, err := r.authorizedProject(ctx, &resource)
	if project == nil {
		return result, err
	}
	creds, result, err := r.credentials(ctx, &resource, project)
	if creds == nil {
		return result, err
	}

	desired, err := Desired(&resource)
	if err != nil {
		return ctrl.Result{}, err
	}
	name, _ := desired["name"].(string)
	if !selectelName.MatchString(name) {
		return r.fail(ctx, &resource, cdnv1alpha1.PhasePending, reasonInvalidName,
			"Selectel resource name must match ^[a-zA-Zа-яА-ЯёЁ0-9\\-_ ]{1,50}$", 0)
	}

	if resource.Status.ID == "" {
		if r.rejectedThisGeneration(&resource) {
			return ctrl.Result{}, nil
		}
		if err := r.ensureID(ctx, project, creds, &resource, name, !observing(&resource)); err != nil {
			return r.callResult(ctx, &resource, err)
		}
		if resource.Status.ID == "" {
			if cond := findCondition(resource.Status.Conditions, cdnv1alpha1.ConditionReady); cond != nil && cond.Reason == reasonWouldCreate {
				return ctrl.Result{RequeueAfter: requeueDenied}, nil
			}
			return ctrl.Result{}, nil
		}
	}
	return r.sync(ctx, project, creds, &resource, desired)
}

func observing(resource *cdnv1alpha1.Resource) bool {
	return resource.Spec.ManagementPolicy == cdnv1alpha1.ManagementPolicyObserve
}

func (r *ResourceReconciler) authorizedProject(ctx context.Context, resource *cdnv1alpha1.Resource) (*selectelv1alpha1.Project, ctrl.Result, error) {
	var project selectelv1alpha1.Project
	if err := r.Get(ctx, types.NamespacedName{Name: resource.Spec.ProjectRef.Name}, &project); err != nil {
		if apierrors.IsNotFound(err) {
			result, statusErr := r.fail(ctx, resource, cdnv1alpha1.PhasePending, reasonProjectNotFound, "Project not found", requeueDenied)
			return nil, result, statusErr
		}
		return nil, ctrl.Result{}, err
	}
	entry, ok := access.Lookup(project.Spec.AllowedNamespaces, resource.Namespace)
	if !ok {
		result, err := r.fail(ctx, resource, cdnv1alpha1.PhasePending, reasonNamespaceNotAllowed,
			"namespace is not allowed to use this Project", requeueDenied)
		return nil, result, err
	}
	if resource.Spec.Certificate != nil {
		if resource.Spec.Certificate.ID == nil || !access.AllowsCertificate(entry, *resource.Spec.Certificate.ID) {
			result, err := r.fail(ctx, resource, cdnv1alpha1.PhasePending, reasonCertificateNotAllowed,
				"certificate seq is not allowed for this namespace", requeueDenied)
			return nil, result, err
		}
	}
	ready := findCondition(project.Status.Conditions, selectelv1alpha1.ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionTrue {
		result, err := r.fail(ctx, resource, cdnv1alpha1.PhasePending, reasonProjectNotReady, "Project is not ready", requeueDenied)
		return nil, result, err
	}
	if err := access.AllowsDomains(project.Spec.AllowedDomains, resource.Spec.Names); err != nil {
		reason := reasonDomainNotAllowed
		if access.IsPatternError(err) {
			reason = reasonDomainPatternInvalid
		}
		result, statusErr := r.fail(ctx, resource, cdnv1alpha1.PhasePending, reason, err.Error(), requeueDenied)
		return nil, result, statusErr
	}
	return &project, ctrl.Result{}, nil
}

func (r *ResourceReconciler) credentials(ctx context.Context, resource *cdnv1alpha1.Resource, project *selectelv1alpha1.Project) (*iam.Credentials, ctrl.Result, error) {
	ref := project.Spec.CredentialsSecretRef
	if ref.Namespace != r.CredentialsNamespace {
		result, err := r.fail(ctx, resource, cdnv1alpha1.PhasePending, reasonSecretNamespaceForbidden,
			"Project credentials secret is outside the operator namespace", 0)
		return nil, result, err
	}
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			result, statusErr := r.fail(ctx, resource, cdnv1alpha1.PhasePending, reasonSecretNotFound, "credentials secret was not found", requeueDenied)
			return nil, result, statusErr
		}
		return nil, ctrl.Result{}, err
	}
	creds, err := iam.CredentialsFromSecret(&secret)
	if err != nil {
		result, statusErr := r.fail(ctx, resource, cdnv1alpha1.PhasePending, reasonSecretInvalid, err.Error(), 0)
		return nil, result, statusErr
	}
	return &creds, ctrl.Result{}, nil
}

func (r *ResourceReconciler) rejectedThisGeneration(resource *cdnv1alpha1.Resource) bool {
	cond := findCondition(resource.Status.Conditions, cdnv1alpha1.ConditionReady)
	if cond == nil || cond.ObservedGeneration != resource.Generation {
		return false
	}
	return cond.Reason == reasonCreateRejected || cond.Reason == reasonAdoptFailed
}

func (r *ResourceReconciler) ensureID(ctx context.Context, project *selectelv1alpha1.Project, creds *iam.Credentials, resource *cdnv1alpha1.Resource, name string, allowCreate bool) error {
	if resource.Spec.ResourceID != "" {
		doc, err := r.get(ctx, project, creds, resource.Spec.ResourceID)
		if err != nil {
			if errors.Is(err, cdn.ErrNotFound) {
				_, statusErr := r.fail(ctx, resource, cdnv1alpha1.PhaseError, reasonAdoptFailed, "Selectel resource was not found", 0)
				return statusErr
			}
			return err
		}
		return r.saveIdentity(ctx, resource, resource.Spec.ResourceID, stringField(doc, "cdn_domain"), "", doc)
	}

	items, err := r.list(ctx, project, creds)
	if err != nil {
		return err
	}
	for _, item := range items {
		if stringField(item, "name") == name && stringField(item, "id") != "" {
			return r.saveIdentity(ctx, resource, stringField(item, "id"), stringField(item, "cdn_domain"), "", item)
		}
	}
	if !allowCreate {
		_, statusErr := r.fail(ctx, resource, cdnv1alpha1.PhasePending, reasonWouldCreate, "Manage would create this CDN resource", requeueDenied)
		return statusErr
	}

	body, err := json.Marshal(mustDesired(resource))
	if err != nil {
		return err
	}
	mutation, err := r.create(ctx, project, creds, body)
	if err != nil {
		return err
	}
	if mutation.Status != "accept" || mutation.ResourceID == "" {
		message := mutation.Message
		if mutation.Description != "" {
			if message != "" {
				message += ": "
			}
			message += mutation.Description
		}
		if message == "" {
			message = "Selectel rejected the create request"
		}
		_, statusErr := r.fail(ctx, resource, cdnv1alpha1.PhaseError, reasonCreateRejected, message, 0)
		return statusErr
	}
	return r.saveIdentity(ctx, resource, mutation.ResourceID, mutation.CDNDomain, mutation.TaskID, nil)
}

func (r *ResourceReconciler) sync(ctx context.Context, project *selectelv1alpha1.Project, creds *iam.Credentials, resource *cdnv1alpha1.Resource, desired map[string]any) (ctrl.Result, error) {
	status, err := r.status(ctx, project, creds, resource.Status.ID)
	if err != nil {
		if errors.Is(err, cdn.ErrNotFound) {
			return r.missing(ctx, project, creds, resource, desired)
		}
		return r.callResult(ctx, resource, err)
	}

	switch status.Status {
	case "Processing":
		message := status.Message
		if message == "" {
			message = "Selectel is applying the CDN configuration"
		}
		if err := r.finish(ctx, resource, cdnv1alpha1.PhaseProcessing, metav1.ConditionFalse, reasonProcessing, message, resource.Status.Drift); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: r.processingDelay()}, nil
	case "Error":
		actual, err := r.get(ctx, project, creds, resource.Status.ID)
		if err != nil {
			if errors.Is(err, cdn.ErrNotFound) {
				return r.missing(ctx, project, creds, resource, desired)
			}
			return r.callResult(ctx, resource, err)
		}
		applyObserved(resource, actual)
		matched, err := Matches(desired, actual)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !matched {
			if observing(resource) {
				return r.observe(ctx, resource, desired, actual)
			}
			return r.submitUpdate(ctx, project, creds, resource, desired, actual)
		}
		if observing(resource) {
			return r.observe(ctx, resource, desired, actual)
		}
		message := status.Message
		if message == "" {
			message = "Selectel reported an error"
		}
		if err := r.mark(ctx, resource, cdnv1alpha1.PhaseError, metav1.ConditionFalse, reasonSelectelError, message); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueError}, nil
	case "Completed":
		actual, err := r.get(ctx, project, creds, resource.Status.ID)
		if err != nil {
			if errors.Is(err, cdn.ErrNotFound) {
				return r.missing(ctx, project, creds, resource, desired)
			}
			return r.callResult(ctx, resource, err)
		}
		applyObserved(resource, actual)
		matched, err := Matches(desired, actual)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !matched {
			if observing(resource) {
				return r.observe(ctx, resource, desired, actual)
			}
			return r.submitUpdate(ctx, project, creds, resource, desired, actual)
		}
		if observing(resource) {
			return r.observe(ctx, resource, desired, actual)
		}
		if err := r.mark(ctx, resource, cdnv1alpha1.PhaseReady, metav1.ConditionTrue, reasonSynced, "CDN resource matches the spec"); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueReady}, nil
	default:
		if err := r.finish(ctx, resource, cdnv1alpha1.PhaseProcessing, metav1.ConditionFalse, reasonProcessing, "waiting for a Selectel status", resource.Status.Drift); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: r.processingDelay()}, nil
	}
}

func (r *ResourceReconciler) submitUpdate(ctx context.Context, project *selectelv1alpha1.Project, creds *iam.Credentials, resource *cdnv1alpha1.Resource, desired, actual map[string]any) (ctrl.Result, error) {
	if err := r.put(ctx, project, creds, resource, desired); err != nil {
		return r.callResult(ctx, resource, err)
	}
	drift, _, err := Diff(desired, actual)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.finish(ctx, resource, cdnv1alpha1.PhaseProcessing, metav1.ConditionFalse, reasonProcessing, "configuration update submitted", drift); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: r.processingDelay()}, nil
}

func (r *ResourceReconciler) missing(ctx context.Context, project *selectelv1alpha1.Project, creds *iam.Credentials, resource *cdnv1alpha1.Resource, desired map[string]any) (ctrl.Result, error) {
	if observing(resource) {
		return r.fail(ctx, resource, cdnv1alpha1.PhaseError, reasonWouldRecreate, "Selectel resource was not found; Manage would create it again", requeueError)
	}
	return r.recreate(ctx, project, creds, resource, desired)
}

func (r *ResourceReconciler) observe(ctx context.Context, resource *cdnv1alpha1.Resource, desired, actual map[string]any) (ctrl.Result, error) {
	drift, total, err := Diff(desired, actual)
	if err != nil {
		return ctrl.Result{}, err
	}
	if total == 0 {
		if err := r.mark(ctx, resource, cdnv1alpha1.PhaseReady, metav1.ConditionTrue, reasonObserving, "spec matches and managementPolicy is Observe"); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueReady}, nil
	}
	message := fmt.Sprintf("Manage would update %d fields", total)
	if err := r.finish(ctx, resource, cdnv1alpha1.PhaseDrift, metav1.ConditionFalse, reasonDrift, message, drift); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeueReady}, nil
}

func (r *ResourceReconciler) recreate(ctx context.Context, project *selectelv1alpha1.Project, creds *iam.Credentials, resource *cdnv1alpha1.Resource, desired map[string]any) (ctrl.Result, error) {
	logf.FromContext(ctx).Info("Selectel resource disappeared, creating it again", "id", resource.Status.ID)
	r.event(resource, "Recreated", "Selectel resource was not found and will be created again")
	resource.Status.ID = ""
	resource.Status.CDNDomain = ""
	resource.Status.TaskID = ""
	resource.Status.CreationTimestamp = nil
	name, _ := desired["name"].(string)
	if err := r.ensureID(ctx, project, creds, resource, name, true); err != nil {
		return r.callResult(ctx, resource, err)
	}
	if resource.Status.ID == "" {
		return ctrl.Result{}, nil
	}
	return r.sync(ctx, project, creds, resource, desired)
}

func (r *ResourceReconciler) finalize(ctx context.Context, resource *cdnv1alpha1.Resource) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(resource, cdnv1alpha1.ResourceFinalizer) {
		return ctrl.Result{}, nil
	}
	policy := resource.Spec.DeletionPolicy
	if policy == "" {
		policy = cdnv1alpha1.DeletionPolicyDeactivate
	}
	if policy == cdnv1alpha1.DeletionPolicyDeactivate && resource.Status.ID != "" && !observing(resource) {
		if err := r.deactivate(ctx, resource); err != nil {
			return r.callResult(ctx, resource, err)
		}
	}
	controllerutil.RemoveFinalizer(resource, cdnv1alpha1.ResourceFinalizer)
	return ctrl.Result{}, r.Update(ctx, resource)
}

func (r *ResourceReconciler) deactivate(ctx context.Context, resource *cdnv1alpha1.Resource) error {
	var project selectelv1alpha1.Project
	if err := r.Get(ctx, types.NamespacedName{Name: resource.Spec.ProjectRef.Name}, &project); err != nil {
		return err
	}
	if project.Spec.CredentialsSecretRef.Namespace != r.CredentialsNamespace {
		return fmt.Errorf("credentials secret is outside the operator namespace")
	}
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Namespace: project.Spec.CredentialsSecretRef.Namespace, Name: project.Spec.CredentialsSecretRef.Name}, &secret); err != nil {
		return err
	}
	creds, err := iam.CredentialsFromSecret(&secret)
	if err != nil {
		return err
	}
	_, err = r.patch(ctx, &project, &creds, resource.Status.ID, []byte(`{"active":false}`))
	if errors.Is(err, cdn.ErrNotFound) {
		return nil
	}
	return err
}

func (r *ResourceReconciler) callResult(ctx context.Context, resource *cdnv1alpha1.Resource, err error) (ctrl.Result, error) {
	if err == nil {
		return ctrl.Result{}, nil
	}
	var limited *cdn.RateLimitError
	if errors.As(err, &limited) {
		wait := limited.RetryAfter
		if wait <= 0 {
			wait = 30 * time.Second
		}
		if statusErr := r.mark(ctx, resource, resource.Status.Phase, metav1.ConditionFalse, reasonRateLimited, "Selectel rate limited the request"); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{RequeueAfter: wait}, nil
	}
	if errors.Is(err, cdn.ErrUnauthorized) {
		if statusErr := r.mark(ctx, resource, resource.Status.Phase, metav1.ConditionFalse, reasonAuthFailed, "Selectel rejected the IAM token"); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{RequeueAfter: requeueError}, nil
	}
	if statusErr := r.mark(ctx, resource, resource.Status.Phase, metav1.ConditionFalse, reasonAPIError, err.Error()); statusErr != nil {
		return ctrl.Result{}, statusErr
	}
	return ctrl.Result{RequeueAfter: requeueError}, nil
}

func (r *ResourceReconciler) fail(ctx context.Context, resource *cdnv1alpha1.Resource, phase cdnv1alpha1.ResourcePhase, reason, message string, after time.Duration) (ctrl.Result, error) {
	if err := r.mark(ctx, resource, phase, metav1.ConditionFalse, reason, message); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: after}, nil
}

func (r *ResourceReconciler) mark(ctx context.Context, resource *cdnv1alpha1.Resource, phase cdnv1alpha1.ResourcePhase, status metav1.ConditionStatus, reason, message string) error {
	return r.finish(ctx, resource, phase, status, reason, message, nil)
}

func (r *ResourceReconciler) finish(ctx context.Context, resource *cdnv1alpha1.Resource, phase cdnv1alpha1.ResourcePhase, status metav1.ConditionStatus, reason, message string, drift []cdnv1alpha1.FieldDrift) error {
	if len(drift) == 0 {
		drift = nil
	}
	driftChanged := !sameDrift(resource.Status.Drift, drift)
	phaseChanged := resource.Status.Phase != phase
	resource.Status.Phase = phase
	resource.Status.Drift = drift
	conditionChanged := setCondition(&resource.Status.Conditions, resource.Generation, cdnv1alpha1.ConditionReady, status, reason, message)
	if !phaseChanged && !conditionChanged && !driftChanged {
		return nil
	}
	return r.Status().Update(ctx, resource)
}

func sameDrift(left, right []cdnv1alpha1.FieldDrift) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (r *ResourceReconciler) saveIdentity(ctx context.Context, resource *cdnv1alpha1.Resource, id, domain, taskID string, doc map[string]any) error {
	resource.Status.ID = id
	if domain != "" {
		resource.Status.CDNDomain = domain
	}
	if taskID != "" {
		resource.Status.TaskID = taskID
	}
	if doc != nil {
		applyObserved(resource, doc)
	}
	return r.mark(ctx, resource, cdnv1alpha1.PhasePending, metav1.ConditionFalse, reasonProcessing, "Selectel resource id recorded")
}

func applyObserved(resource *cdnv1alpha1.Resource, doc map[string]any) {
	if domain := stringField(doc, "cdn_domain"); domain != "" {
		resource.Status.CDNDomain = domain
	}
	if created, ok := doc["creation_ts"].(float64); ok && created > 0 {
		stamp := metav1.NewTime(time.Unix(int64(created), 0).UTC())
		resource.Status.CreationTimestamp = &stamp
	}
}

func (r *ResourceReconciler) put(ctx context.Context, project *selectelv1alpha1.Project, creds *iam.Credentials, resource *cdnv1alpha1.Resource, desired map[string]any) error {
	body, err := json.Marshal(desired)
	if err != nil {
		return err
	}
	mutation, err := r.putCall(ctx, project, creds, resource.Status.ID, body)
	if err != nil {
		return err
	}
	if mutation.Status == "error" {
		return fmt.Errorf("selectel update rejected: %s", mutationMessage(mutation))
	}
	if mutation.TaskID != "" {
		resource.Status.TaskID = mutation.TaskID
	}
	return nil
}

func mutationMessage(mutation cdn.Mutation) string {
	if mutation.Description != "" && mutation.Message != "" {
		return mutation.Message + ": " + mutation.Description
	}
	if mutation.Description != "" {
		return mutation.Description
	}
	if mutation.Message != "" {
		return mutation.Message
	}
	return mutation.Status
}

func mustDesired(resource *cdnv1alpha1.Resource) map[string]any {
	doc, err := Desired(resource)
	if err != nil {
		return map[string]any{}
	}
	return doc
}

func stringField(doc map[string]any, key string) string {
	value, _ := doc[key].(string)
	return value
}

func (r *ResourceReconciler) event(resource *cdnv1alpha1.Resource, reason, message string) {
	if r.Recorder == nil {
		return
	}
	r.Recorder.Eventf(resource, nil, corev1.EventTypeNormal, reason, "Reconcile", "%s", message)
}

func (r *ResourceReconciler) token(ctx context.Context, project *selectelv1alpha1.Project, creds *iam.Credentials) (string, error) {
	return r.Tokens.Token(ctx, project.Name, project.Spec.AuthURL, *creds)
}

func (r *ResourceReconciler) withToken(ctx context.Context, project *selectelv1alpha1.Project, creds *iam.Credentials, fn func(token string) error) error {
	token, err := r.token(ctx, project, creds)
	if err != nil {
		return err
	}
	err = fn(token)
	if !errors.Is(err, cdn.ErrUnauthorized) {
		return err
	}
	r.Tokens.Invalidate(project.Name)
	token, err = r.token(ctx, project, creds)
	if err != nil {
		return err
	}
	return fn(token)
}

func (r *ResourceReconciler) get(ctx context.Context, project *selectelv1alpha1.Project, creds *iam.Credentials, id string) (map[string]any, error) {
	var doc map[string]any
	err := r.withToken(ctx, project, creds, func(token string) error {
		var callErr error
		doc, callErr = r.CDN.Get(ctx, token, id)
		return callErr
	})
	return doc, err
}

func (r *ResourceReconciler) list(ctx context.Context, project *selectelv1alpha1.Project, creds *iam.Credentials) ([]map[string]any, error) {
	var items []map[string]any
	err := r.withToken(ctx, project, creds, func(token string) error {
		var callErr error
		items, callErr = r.CDN.List(ctx, token)
		return callErr
	})
	return items, err
}

func (r *ResourceReconciler) create(ctx context.Context, project *selectelv1alpha1.Project, creds *iam.Credentials, body []byte) (cdn.Mutation, error) {
	var mutation cdn.Mutation
	err := r.withToken(ctx, project, creds, func(token string) error {
		var callErr error
		mutation, callErr = r.CDN.Create(ctx, token, body)
		return callErr
	})
	return mutation, err
}

func (r *ResourceReconciler) putCall(ctx context.Context, project *selectelv1alpha1.Project, creds *iam.Credentials, id string, body []byte) (cdn.Mutation, error) {
	var mutation cdn.Mutation
	err := r.withToken(ctx, project, creds, func(token string) error {
		var callErr error
		mutation, callErr = r.CDN.Put(ctx, token, id, body)
		return callErr
	})
	return mutation, err
}

func (r *ResourceReconciler) patch(ctx context.Context, project *selectelv1alpha1.Project, creds *iam.Credentials, id string, body []byte) (cdn.Mutation, error) {
	var mutation cdn.Mutation
	err := r.withToken(ctx, project, creds, func(token string) error {
		var callErr error
		mutation, callErr = r.CDN.Patch(ctx, token, id, body)
		return callErr
	})
	return mutation, err
}

func (r *ResourceReconciler) status(ctx context.Context, project *selectelv1alpha1.Project, creds *iam.Credentials, id string) (cdn.Status, error) {
	var status cdn.Status
	err := r.withToken(ctx, project, creds, func(token string) error {
		var callErr error
		status, callErr = r.CDN.Status(ctx, token, id)
		return callErr
	})
	return status, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cdnv1alpha1.Resource{}).
		Watches(&selectelv1alpha1.Project{}, handler.EnqueueRequestsFromMapFunc(r.projectToResources)).
		Named("cdn.selectel-resource").
		Complete(r)
}

func (r *ResourceReconciler) projectToResources(ctx context.Context, obj client.Object) []reconcile.Request {
	project, ok := obj.(*selectelv1alpha1.Project)
	if !ok {
		return nil
	}
	var resources cdnv1alpha1.ResourceList
	if err := r.List(ctx, &resources); err != nil {
		return nil
	}
	var requests []reconcile.Request
	for i := range resources.Items {
		if resources.Items[i].Spec.ProjectRef.Name == project.Name {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: resources.Items[i].Namespace,
				Name:      resources.Items[i].Name,
			}})
		}
	}
	return requests
}

func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

func setCondition(conditions *[]metav1.Condition, generation int64, condType string, status metav1.ConditionStatus, reason, message string) bool {
	now := metav1.Now()
	for i := range *conditions {
		existing := &(*conditions)[i]
		if existing.Type != condType {
			continue
		}
		if existing.Status == status && existing.Reason == reason && existing.Message == message && existing.ObservedGeneration == generation {
			return false
		}
		if existing.Status != status {
			existing.LastTransitionTime = now
		}
		existing.Status = status
		existing.Reason = reason
		existing.Message = message
		existing.ObservedGeneration = generation
		return true
	}
	*conditions = append(*conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
		LastTransitionTime: now,
	})
	return true
}
