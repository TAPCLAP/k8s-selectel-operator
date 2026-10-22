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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdnv1alpha1 "github.com/tapclap/k8s-selectel-operator/api/cdn.selectel/v1alpha1"
)

func TestDesiredStripsOperatorFieldsAndUsesCertificateID(t *testing.T) {
	t.Parallel()
	certID := int32(4)
	https := true
	resource := &cdnv1alpha1.Resource{
		ObjectMeta: metav1.ObjectMeta{Name: "static"},
		Spec: cdnv1alpha1.ResourceSpec{
			ProjectRef:       cdnv1alpha1.ProjectRef{Name: "production"},
			DeletionPolicy:   cdnv1alpha1.DeletionPolicyOrphan,
			ManagementPolicy: cdnv1alpha1.ManagementPolicyObserve,
			ResourceID:       "already",
			Certificate:      &cdnv1alpha1.CertificateSource{ID: &certID},
			Origin: &cdnv1alpha1.Origin{
				HTTPS:   &https,
				Servers: map[string]cdnv1alpha1.OriginServer{"origin.example.com": {}},
			},
		},
	}
	got, err := Desired(resource)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["projectRef"]; ok {
		t.Fatal("projectRef leaked into the API body")
	}
	if _, ok := got["deletionPolicy"]; ok || got["resourceID"] != nil || got["managementPolicy"] != nil {
		t.Fatal("operator fields leaked")
	}
	if got["name"] != "static" {
		t.Fatalf("name = %v", got["name"])
	}
	if got["certificate"] != certID {
		t.Fatalf("certificate = %#v", got["certificate"])
	}
}

func TestMatchesIgnoresServerDefaults(t *testing.T) {
	t.Parallel()
	desired := map[string]any{"name": "static", "origin": map[string]any{"https": true}}
	actual := map[string]any{
		"id":         "abc",
		"cdn_domain": "abc.selcdn.net",
		"name":       "static",
		"origin":     map[string]any{"https": true, "connect_timeout": "5s"},
	}
	ok, err := Matches(desired, actual)
	if err != nil || !ok {
		t.Fatalf("match = %v %v", ok, err)
	}
	actual["origin"].(map[string]any)["https"] = false
	ok, err = Matches(desired, actual)
	if err != nil || ok {
		t.Fatalf("expected drift, match = %v %v", ok, err)
	}
}

func TestDiffReportsNestedFieldsAndIgnoresDefaults(t *testing.T) {
	t.Parallel()
	desired := map[string]any{"name": "static", "origin": map[string]any{"https": true}}
	actual := map[string]any{
		"id":     "abc",
		"name":   "static",
		"origin": map[string]any{"https": false, "connect_timeout": "5s"},
	}
	got, total, err := Diff(desired, actual)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(got) != 1 || got[0].Path != "origin.https" {
		t.Fatalf("drift = %#v total %d", got, total)
	}
}
