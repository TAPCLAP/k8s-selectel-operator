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
	"net/http"
	"net/http/httptest"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdnv1alpha1 "github.com/tapclap/k8s-selectel-operator/api/cdn.selectel/v1alpha1"
	selectelv1alpha1 "github.com/tapclap/k8s-selectel-operator/api/selectel/v1alpha1"
	"github.com/tapclap/k8s-selectel-operator/internal/selectel/iam"
)

var _ = Describe("Project controller", func() {
	var (
		hits      atomic.Int32
		iamServer *httptest.Server
		rec       *ProjectReconciler
	)

	BeforeEach(func() {
		hits.Store(0)
		iamServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			w.Header().Set("X-Subject-Token", "token")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"token":{"expires_at":"2099-01-01T00:00:00Z"}}`))
		}))
		rec = &ProjectReconciler{
			Client:               k8sClient,
			Scheme:               scheme.Scheme,
			CredentialsNamespace: "selectel-infra",
			Tokens:               iam.NewCache(iam.NewClient(iamServer.Client())),
		}
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "selectel-infra"}}
		err := k8sClient.Create(ctx, ns)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}
		media := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "media"}}
		err = k8sClient.Create(ctx, media)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	AfterEach(func() {
		iamServer.Close()
		var resources cdnv1alpha1.ResourceList
		Expect(k8sClient.List(ctx, &resources)).To(Succeed())
		for i := range resources.Items {
			item := &resources.Items[i]
			item.Finalizers = nil
			_ = k8sClient.Update(ctx, item)
			_ = k8sClient.Delete(ctx, item)
		}
		var projects selectelv1alpha1.ProjectList
		Expect(k8sClient.List(ctx, &projects)).To(Succeed())
		for i := range projects.Items {
			item := &projects.Items[i]
			item.Finalizers = nil
			_ = k8sClient.Update(ctx, item)
			_ = k8sClient.Delete(ctx, item)
		}
		var secrets corev1.SecretList
		Expect(k8sClient.List(ctx, &secrets, client.InNamespace("selectel-infra"))).To(Succeed())
		for i := range secrets.Items {
			_ = k8sClient.Delete(ctx, &secrets.Items[i])
		}
		var other corev1.SecretList
		Expect(k8sClient.List(ctx, &other, client.InNamespace("media"))).To(Succeed())
		for i := range other.Items {
			_ = k8sClient.Delete(ctx, &other.Items[i])
		}
	})

	It("does not call IAM when the secret is missing", func() {
		createProject(iamServer.URL, "selectel-infra", "missing")
		_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "production"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(hits.Load()).To(Equal(int32(0)))
		Expect(projectCondition().Reason).To(Equal(reasonSecretNotFound))
	})

	It("does not read a secret outside the operator namespace", func() {
		Expect(k8sClient.Create(ctx, credentialSecret("media", "selectel"))).To(Succeed())
		createProject(iamServer.URL, "media", "selectel")
		_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "production"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(hits.Load()).To(Equal(int32(0)))
		Expect(projectCondition().Reason).To(Equal(reasonSecretNamespaceForbidden))
	})

	It("issues a token once and keeps the finalizer while a resource exists", func() {
		Expect(k8sClient.Create(ctx, credentialSecret("selectel-infra", "selectel"))).To(Succeed())
		createProject(iamServer.URL, "selectel-infra", "selectel")
		_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "production"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(hits.Load()).To(Equal(int32(1)))
		Expect(projectCondition().Status).To(Equal(metav1.ConditionTrue))

		_, err = rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "production"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(hits.Load()).To(Equal(int32(1)))

		https := true
		Expect(k8sClient.Create(ctx, &cdnv1alpha1.Resource{
			ObjectMeta: metav1.ObjectMeta{Name: "static", Namespace: "media"},
			Spec: cdnv1alpha1.ResourceSpec{
				ProjectRef: cdnv1alpha1.ProjectRef{Name: "production"},
				Origin:     &cdnv1alpha1.Origin{HTTPS: &https, Servers: map[string]cdnv1alpha1.OriginServer{"origin.example.com": {}}},
			},
		})).To(Succeed())

		Expect(k8sClient.Delete(ctx, &selectelv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "production"}})).To(Succeed())
		_, err = rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "production"}})
		Expect(err).NotTo(HaveOccurred())
		var project selectelv1alpha1.Project
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "production"}, &project)).To(Succeed())
		Expect(project.Finalizers).To(ContainElement(selectelv1alpha1.ProjectFinalizer))

		var resource cdnv1alpha1.Resource
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "static", Namespace: "media"}, &resource)).To(Succeed())
		resource.Finalizers = nil
		Expect(k8sClient.Update(ctx, &resource)).To(Succeed())
		Expect(k8sClient.Delete(ctx, &resource)).To(Succeed())

		_, err = rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "production"}})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.Get(ctx, types.NamespacedName{Name: "production"}, &selectelv1alpha1.Project{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})
})

func createProject(authURL, secretNamespace, secretName string) {
	Expect(k8sClient.Create(ctx, &selectelv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "production"},
		Spec: selectelv1alpha1.ProjectSpec{
			AuthURL: authURL,
			CredentialsSecretRef: selectelv1alpha1.SecretReference{
				Name:      secretName,
				Namespace: secretNamespace,
			},
		},
	})).To(Succeed())
}

func credentialSecret(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data: map[string][]byte{
			iam.KeyAccountID:   []byte("123"),
			iam.KeyUsername:    []byte("user"),
			iam.KeyPassword:    []byte("pass"),
			iam.KeyProjectName: []byte("prod"),
		},
	}
}

func projectCondition() metav1.Condition {
	var project selectelv1alpha1.Project
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "production"}, &project)).To(Succeed())
	for _, cond := range project.Status.Conditions {
		if cond.Type == selectelv1alpha1.ConditionReady {
			return cond
		}
	}
	Fail("ready condition not found")
	return metav1.Condition{}
}
