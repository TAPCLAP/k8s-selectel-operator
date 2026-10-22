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
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/google/uuid"
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
	"github.com/tapclap/k8s-selectel-operator/internal/selectel/cdn"
	"github.com/tapclap/k8s-selectel-operator/internal/selectel/iam"
)

const (
	driftedOriginHost = "other.example.com"
	mediaNamespace    = "media"
)

type cdnFake struct {
	mu          sync.Mutex
	items       map[string]map[string]any
	phase       string
	createError bool
	posts       int
	puts        int
	patches     int
	lastPost    map[string]any
	lastPatch   map[string]any
}

func newCDNFake() *cdnFake {
	return &cdnFake{items: map[string]map[string]any{}, phase: "Completed"}
}

func (f *cdnFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/")
	switch {
	case r.Method == http.MethodPost && path == "resources":
		f.posts++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.lastPost = body
		if f.createError {
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Json invalid", "description": "origin"})
			return
		}
		id := uuid.NewString()
		body["id"] = id
		body["cdn_domain"] = id + ".selcdn.net"
		body["creation_ts"] = float64(1700000000)
		f.items[id] = body
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":      "accept",
			"resource_id": id,
			"cdn_domain":  id + ".selcdn.net",
			"task_id":     "task-1",
		})
	case r.Method == http.MethodGet && path == "resources":
		items := make([]map[string]any, 0, len(f.items))
		for _, item := range f.items {
			items = append(items, item)
		}
		_ = json.NewEncoder(w).Encode(items)
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/status"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "resources/"), "/status")
		if _, ok := f.items[id]; !ok {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": f.phase, "message": "configuration processing", "resource": id})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "resources/"):
		id := strings.TrimPrefix(path, "resources/")
		item, ok := f.items[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(item)
	case r.Method == http.MethodPut && strings.HasPrefix(path, "resources/"):
		f.puts++
		id := strings.TrimPrefix(path, "resources/")
		if _, ok := f.items[id]; !ok {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		body["id"] = id
		body["cdn_domain"] = id + ".selcdn.net"
		f.items[id] = body
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "accept", "task_id": "task-2"})
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "resources/"):
		f.patches++
		id := strings.TrimPrefix(path, "resources/")
		item, ok := f.items[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var patch map[string]any
		_ = json.Unmarshal(raw, &patch)
		f.lastPatch = patch
		maps.Copy(item, patch)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "accept"})
	default:
		http.NotFound(w, r)
	}
}

func (f *cdnFake) seed(id string, body map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body["id"] = id
	body["cdn_domain"] = id + ".selcdn.net"
	f.items[id] = body
}

func (f *cdnFake) drop(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.items, id)
}

var _ = Describe("Resource controller", func() {
	const namespace = mediaNamespace

	var (
		fake      *cdnFake
		cdnServer *httptest.Server
		iamServer *httptest.Server
		rec       *ResourceReconciler
	)

	BeforeEach(func() {
		fake = newCDNFake()
		cdnServer = httptest.NewServer(fake)
		iamServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Subject-Token", "token")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"token":{"expires_at":"2099-01-01T00:00:00Z"}}`))
		}))
		cdnClient, err := cdn.New(cdnServer.URL, cdnServer.Client())
		Expect(err).NotTo(HaveOccurred())
		rec = &ResourceReconciler{
			Client:               k8sClient,
			Scheme:               scheme.Scheme,
			CredentialsNamespace: "selectel-infra",
			Tokens:               iam.NewCache(iam.NewClient(iamServer.Client())),
			CDN:                  cdnClient,
		}
		currentReconciler = rec
		ensureNamespace("selectel-infra")
		ensureNamespace(namespace)
		ensureNamespace("games")
		createSecret()
		createReadyProject(iamServer.URL, []selectelv1alpha1.NamespaceAccess{{
			Name:         namespace,
			Certificates: []selectelv1alpha1.AllowedCertificate{{ID: 1}, {ID: 4}},
		}})
	})

	AfterEach(func() {
		cdnServer.Close()
		iamServer.Close()
		wipe()
	})

	It("creates once and updates only when the spec drifts", func() {
		createResource("static", nil)
		runReconcile("static")
		Expect(fake.posts).To(Equal(1))
		Expect(fake.puts).To(Equal(0))
		created := getResource("static")
		Expect(created.Status.ID).NotTo(BeEmpty())
		Expect(created.Status.CDNDomain).To(HaveSuffix(".selcdn.net"))
		Expect(created.Status.Phase).To(Equal(cdnv1alpha1.PhaseReady))

		runReconcile("static")
		Expect(fake.posts).To(Equal(1))
		Expect(fake.puts).To(Equal(0))

		fresh := getResource("static")
		host := driftedOriginHost
		fresh.Spec.Origin.Hostname = &host
		Expect(k8sClient.Update(ctx, &fresh)).To(Succeed())
		runReconcile("static")
		Expect(fake.puts).To(Equal(1))
		Expect(getResource("static").Status.Drift).NotTo(BeEmpty())
	})

	It("does not put while Selectel is still processing", func() {
		fake.phase = "Processing"
		createResource("static", nil)
		runReconcile("static")
		Expect(fake.posts).To(Equal(1))
		Expect(getResource("static").Status.Phase).To(Equal(cdnv1alpha1.PhaseProcessing))

		fresh := getResource("static")
		host := driftedOriginHost
		fresh.Spec.Origin.Hostname = &host
		Expect(k8sClient.Update(ctx, &fresh)).To(Succeed())
		runReconcile("static")
		Expect(fake.puts).To(Equal(0))
	})

	It("deactivates on delete and can orphan instead", func() {
		createResource("static", nil)
		runReconcile("static")
		Expect(k8sClient.Delete(ctx, &cdnv1alpha1.Resource{ObjectMeta: metav1.ObjectMeta{Name: "static", Namespace: namespace}})).To(Succeed())
		runReconcile("static")
		Expect(fake.patches).To(Equal(1))
		Expect(fake.lastPatch["active"]).To(BeFalse())
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "static", Namespace: namespace}, &cdnv1alpha1.Resource{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())

		policy := cdnv1alpha1.DeletionPolicyOrphan
		createResource("orphan", &policy)
		runReconcile("orphan")
		Expect(k8sClient.Delete(ctx, &cdnv1alpha1.Resource{ObjectMeta: metav1.ObjectMeta{Name: "orphan", Namespace: namespace}})).To(Succeed())
		runReconcile("orphan")
		Expect(fake.patches).To(Equal(1))
	})

	It("adopts an existing resource id", func() {
		const id = "6d19b7a2-a493-48fa-a9ec-aea7ba9ffa28"
		fake.seed(id, map[string]any{"name": "static"})
		res := resourceSpec("static", nil)
		res.Spec.ResourceID = id
		Expect(k8sClient.Create(ctx, res)).To(Succeed())
		runReconcile("static")
		Expect(fake.posts).To(Equal(0))
		Expect(getResource("static").Status.ID).To(Equal(id))
	})

	It("does not create a second resource after a rejected create", func() {
		fake.createError = true
		createResource("static", nil)
		runReconcile("static")
		runReconcile("static")
		Expect(fake.posts).To(Equal(1))
		Expect(condition("static").Reason).To(Equal(reasonCreateRejected))
	})

	It("recreates a resource that disappeared in Selectel", func() {
		createResource("static", nil)
		runReconcile("static")
		id := getResource("static").Status.ID
		fake.drop(id)
		runReconcile("static")
		Expect(fake.posts).To(Equal(2))
		Expect(getResource("static").Status.ID).NotTo(Equal(id))
	})

	It("observes an imported resource, reports drift, and applies it after Manage", func() {
		const id = "6d19b7a2-a493-48fa-a9ec-aea7ba9ffa28"
		fake.seed(id, map[string]any{
			"name": "imported",
			"origin": map[string]any{
				"https":   false,
				"servers": map[string]any{"origin.example.com": map[string]any{}},
			},
		})
		res := resourceSpec("imported", nil)
		res.Spec.ManagementPolicy = cdnv1alpha1.ManagementPolicyObserve
		res.Spec.ResourceID = id
		Expect(k8sClient.Create(ctx, res)).To(Succeed())
		runReconcile("imported")

		Expect(fake.posts).To(Equal(0))
		Expect(fake.puts).To(Equal(0))
		got := getResource("imported")
		Expect(got.Status.ID).To(Equal(id))
		Expect(got.Status.Phase).To(Equal(cdnv1alpha1.PhaseDrift))
		Expect(condition("imported").Reason).To(Equal(reasonDrift))
		Expect(got.Status.Drift).NotTo(BeEmpty())

		fresh := getResource("imported")
		https := true
		fresh.Spec.Origin.HTTPS = &https
		Expect(k8sClient.Update(ctx, &fresh)).To(Succeed())
		fake.seed(id, map[string]any{
			"name": "imported",
			"origin": map[string]any{
				"https":   true,
				"servers": map[string]any{"origin.example.com": map[string]any{}},
			},
		})
		runReconcile("imported")
		Expect(fake.puts).To(Equal(0))
		Expect(getResource("imported").Status.Phase).To(Equal(cdnv1alpha1.PhaseReady))
		Expect(condition("imported").Reason).To(Equal(reasonObserving))
		Expect(getResource("imported").Status.Drift).To(BeEmpty())

		managed := getResource("imported")
		host := driftedOriginHost
		managed.Spec.ManagementPolicy = cdnv1alpha1.ManagementPolicyManage
		managed.Spec.Origin.Hostname = &host
		Expect(k8sClient.Update(ctx, &managed)).To(Succeed())
		runReconcile("imported")
		Expect(fake.posts).To(Equal(0))
		Expect(fake.puts).To(Equal(1))
		Expect(getResource("imported").Status.Drift).NotTo(BeEmpty())
	})

	It("links an existing name while observing and waits when the name is missing", func() {
		const id = "6d19b7a2-a493-48fa-a9ec-aea7ba9ffa28"
		fake.seed(id, map[string]any{
			"name": "auto",
			"origin": map[string]any{
				"https":   false,
				"servers": map[string]any{"origin.example.com": map[string]any{}},
			},
		})
		res := resourceSpec("auto", nil)
		res.Spec.ManagementPolicy = cdnv1alpha1.ManagementPolicyObserve
		Expect(k8sClient.Create(ctx, res)).To(Succeed())
		runReconcile("auto")
		Expect(fake.posts).To(Equal(0))
		Expect(fake.puts).To(Equal(0))
		Expect(getResource("auto").Status.ID).To(Equal(id))
		Expect(getResource("auto").Status.Phase).To(Equal(cdnv1alpha1.PhaseDrift))
		Expect(getResource("auto").Spec.ResourceID).To(BeEmpty())

		waiting := resourceSpec("later", nil)
		waiting.Spec.ManagementPolicy = cdnv1alpha1.ManagementPolicyObserve
		Expect(k8sClient.Create(ctx, waiting)).To(Succeed())
		runReconcile("later")
		Expect(fake.posts).To(Equal(0))
		Expect(getResource("later").Status.ID).To(BeEmpty())
		Expect(condition("later").Reason).To(Equal(reasonWouldCreate))

		fake.seed("7c9e6679-7425-40de-944b-e07fc1f90ae7", map[string]any{
			"name": "later",
			"origin": map[string]any{
				"https":   true,
				"servers": map[string]any{"origin.example.com": map[string]any{}},
			},
		})
		runReconcile("later")
		Expect(fake.posts).To(Equal(0))
		Expect(getResource("later").Status.ID).To(Equal("7c9e6679-7425-40de-944b-e07fc1f90ae7"))
		Expect(condition("later").Reason).To(Equal(reasonObserving))

		Expect(k8sClient.Delete(ctx, &cdnv1alpha1.Resource{ObjectMeta: metav1.ObjectMeta{Name: "auto", Namespace: namespace}})).To(Succeed())
		runReconcile("auto")
		Expect(fake.patches).To(Equal(0))
	})

	It("imports by name and does not create while observing", func() {
		const id = "6d19b7a2-a493-48fa-a9ec-aea7ba9ffa28"
		fake.seed(id, map[string]any{
			"name": "by-name",
			"origin": map[string]any{
				"https":   true,
				"servers": map[string]any{"origin.example.com": map[string]any{}},
			},
		})
		res := resourceSpec("by-name", nil)
		res.Spec.ManagementPolicy = cdnv1alpha1.ManagementPolicyObserve
		Expect(k8sClient.Create(ctx, res)).To(Succeed())
		runReconcile("by-name")
		Expect(fake.posts).To(Equal(0))
		Expect(getResource("by-name").Status.ID).To(Equal(id))
		Expect(condition("by-name").Reason).To(Equal(reasonObserving))

		missing := resourceSpec("absent", nil)
		missing.Spec.ManagementPolicy = cdnv1alpha1.ManagementPolicyObserve
		Expect(k8sClient.Create(ctx, missing)).To(Succeed())
		runReconcile("absent")
		Expect(fake.posts).To(Equal(0))
		Expect(getResource("absent").Status.ID).To(BeEmpty())
		Expect(condition("absent").Reason).To(Equal(reasonWouldCreate))
	})

	It("does not deactivate or recreate while observing", func() {
		const id = "6d19b7a2-a493-48fa-a9ec-aea7ba9ffa28"
		fake.seed(id, map[string]any{"name": "watched"})
		res := resourceSpec("watched", nil)
		res.Spec.ManagementPolicy = cdnv1alpha1.ManagementPolicyObserve
		res.Spec.ResourceID = id
		res.Spec.DeletionPolicy = cdnv1alpha1.DeletionPolicyDeactivate
		Expect(k8sClient.Create(ctx, res)).To(Succeed())
		runReconcile("watched")
		Expect(fake.posts).To(Equal(0))

		fake.drop(id)
		runReconcile("watched")
		Expect(fake.posts).To(Equal(0))
		Expect(condition("watched").Reason).To(Equal(reasonWouldRecreate))

		Expect(k8sClient.Delete(ctx, &cdnv1alpha1.Resource{ObjectMeta: metav1.ObjectMeta{Name: "watched", Namespace: namespace}})).To(Succeed())
		runReconcile("watched")
		Expect(fake.patches).To(Equal(0))
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "watched", Namespace: namespace}, &cdnv1alpha1.Resource{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("refuses a certificate outside the namespace allowlist and allows a resource without one", func() {
		denied := int32(99)
		res := resourceSpec("denied", nil)
		res.Spec.Certificate = &cdnv1alpha1.CertificateSource{ID: &denied}
		Expect(k8sClient.Create(ctx, res)).To(Succeed())
		runReconcile("denied")
		Expect(fake.posts).To(Equal(0))
		Expect(condition("denied").Reason).To(Equal(reasonCertificateNotAllowed))

		createResource("plain", nil)
		runReconcile("plain")
		Expect(fake.posts).To(Equal(1))
		Expect(fake.lastPost).NotTo(HaveKey("certificate"))
	})

	It("does not inherit wildcard certificates into an explicit namespace entry", func() {
		replaceProjectAccess(iamServer.URL, []selectelv1alpha1.NamespaceAccess{
			{Name: "*", Certificates: []selectelv1alpha1.AllowedCertificate{{ID: 1}}},
			{Name: namespace, Certificates: []selectelv1alpha1.AllowedCertificate{{ID: 4}}},
		})
		id := int32(1)
		res := resourceSpec("static", nil)
		res.Spec.Certificate = &cdnv1alpha1.CertificateSource{ID: &id}
		Expect(k8sClient.Create(ctx, res)).To(Succeed())
		runReconcile("static")
		Expect(fake.posts).To(Equal(0))
		Expect(condition("static").Reason).To(Equal(reasonCertificateNotAllowed))

		fresh := getResource("static")
		allowed := int32(4)
		fresh.Spec.Certificate.ID = &allowed
		Expect(k8sClient.Update(ctx, &fresh)).To(Succeed())
		runReconcile("static")
		Expect(fake.posts).To(Equal(1))
	})

	It("blocks a disallowed namespace but still deactivates on delete", func() {
		createResource("static", nil)
		runReconcile("static")
		puts := fake.puts
		replaceProjectAccess(iamServer.URL, []selectelv1alpha1.NamespaceAccess{{Name: "games"}})
		runReconcile("static")
		Expect(fake.puts).To(Equal(puts))
		Expect(fake.posts).To(Equal(1))
		Expect(condition("static").Reason).To(Equal(reasonNamespaceNotAllowed))

		Expect(k8sClient.Delete(ctx, &cdnv1alpha1.Resource{ObjectMeta: metav1.ObjectMeta{Name: "static", Namespace: namespace}})).To(Succeed())
		runReconcile("static")
		Expect(fake.patches).To(Equal(1))
	})
})

func ensureNamespace(name string) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	err := k8sClient.Create(ctx, ns)
	if apierrors.IsAlreadyExists(err) {
		return
	}
	Expect(err).NotTo(HaveOccurred())
}

func createSecret() {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "selectel", Namespace: "selectel-infra"},
		Data: map[string][]byte{
			iam.KeyAccountID:   []byte("123"),
			iam.KeyUsername:    []byte("user"),
			iam.KeyPassword:    []byte("pass"),
			iam.KeyProjectName: []byte("prod"),
		},
	}
	err := k8sClient.Create(ctx, secret)
	if apierrors.IsAlreadyExists(err) {
		return
	}
	Expect(err).NotTo(HaveOccurred())
}

func createReadyProject(authURL string, access []selectelv1alpha1.NamespaceAccess) {
	project := &selectelv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "production"},
		Spec: selectelv1alpha1.ProjectSpec{
			AuthURL:              authURL,
			AllowedNamespaces:    access,
			CredentialsSecretRef: selectelv1alpha1.SecretReference{Name: "selectel", Namespace: "selectel-infra"},
		},
	}
	Expect(k8sClient.Create(ctx, project)).To(Succeed())
	markProjectReady(project)
}

func replaceProjectAccess(authURL string, access []selectelv1alpha1.NamespaceAccess) {
	var project selectelv1alpha1.Project
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "production"}, &project)).To(Succeed())
	project.Spec.AuthURL = authURL
	project.Spec.AllowedNamespaces = access
	Expect(k8sClient.Update(ctx, &project)).To(Succeed())
	markProjectReady(&project)
}

func markProjectReady(project *selectelv1alpha1.Project) {
	project.Status.Conditions = []metav1.Condition{{
		Type:               selectelv1alpha1.ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "TokenIssued",
		Message:            "ok",
		LastTransitionTime: metav1.Now(),
	}}
	Expect(k8sClient.Status().Update(ctx, project)).To(Succeed())
}

func createResource(name string, policy *cdnv1alpha1.DeletionPolicy) {
	Expect(k8sClient.Create(ctx, resourceSpec(name, policy))).To(Succeed())
}

func resourceSpec(name string, policy *cdnv1alpha1.DeletionPolicy) *cdnv1alpha1.Resource {
	https := true
	res := &cdnv1alpha1.Resource{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: mediaNamespace},
		Spec: cdnv1alpha1.ResourceSpec{
			ProjectRef: cdnv1alpha1.ProjectRef{Name: "production"},
			Origin: &cdnv1alpha1.Origin{
				HTTPS:   &https,
				Servers: map[string]cdnv1alpha1.OriginServer{"origin.example.com": {}},
			},
		},
	}
	if policy != nil {
		res.Spec.DeletionPolicy = *policy
	}
	return res
}

func runReconcile(name string) {
	_, err := currentReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: mediaNamespace}})
	Expect(err).NotTo(HaveOccurred())
}

var currentReconciler *ResourceReconciler

func getResource(name string) cdnv1alpha1.Resource {
	var got cdnv1alpha1.Resource
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: mediaNamespace}, &got)).To(Succeed())
	return got
}

func condition(name string) metav1.Condition {
	got := getResource(name)
	for _, cond := range got.Status.Conditions {
		if cond.Type == cdnv1alpha1.ConditionReady {
			return cond
		}
	}
	Fail("ready condition not found")
	return metav1.Condition{}
}

func wipe() {
	var resources cdnv1alpha1.ResourceList
	Expect(k8sClient.List(ctx, &resources)).To(Succeed())
	for i := range resources.Items {
		item := &resources.Items[i]
		if len(item.Finalizers) > 0 {
			item.Finalizers = nil
			_ = k8sClient.Update(ctx, item)
		}
		_ = k8sClient.Delete(ctx, item)
	}
	var projects selectelv1alpha1.ProjectList
	Expect(k8sClient.List(ctx, &projects)).To(Succeed())
	for i := range projects.Items {
		item := &projects.Items[i]
		if len(item.Finalizers) > 0 {
			item.Finalizers = nil
			_ = k8sClient.Update(ctx, item)
		}
		_ = k8sClient.Delete(ctx, item)
	}
	var secrets corev1.SecretList
	Expect(k8sClient.List(ctx, &secrets, client.InNamespace("selectel-infra"))).To(Succeed())
	for i := range secrets.Items {
		_ = k8sClient.Delete(ctx, &secrets.Items[i])
	}
}
