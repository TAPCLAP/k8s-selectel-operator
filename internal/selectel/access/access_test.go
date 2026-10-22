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

package access

import (
	"testing"

	selectelv1alpha1 "github.com/tapclap/k8s-selectel-operator/api/selectel/v1alpha1"
)

func TestLookupPrefersExactNamespaceOverWildcard(t *testing.T) {
	t.Parallel()
	entries := []selectelv1alpha1.NamespaceAccess{
		{Name: "*", Certificates: []selectelv1alpha1.AllowedCertificate{{ID: 1}}},
		{Name: "media", Certificates: []selectelv1alpha1.AllowedCertificate{{ID: 4}}},
	}
	got, ok := Lookup(entries, "media")
	if !ok {
		t.Fatal("expected media")
	}
	if AllowsCertificate(got, 1) {
		t.Fatal("exact namespace entry inherited a certificate from *")
	}
	if !AllowsCertificate(got, 4) {
		t.Fatal("expected certificate 4")
	}

	other, ok := Lookup(entries, "games")
	if !ok || !AllowsCertificate(other, 1) || AllowsCertificate(other, 4) {
		t.Fatalf("wildcard = %+v ok=%v", other, ok)
	}
	if _, ok := Lookup(nil, "media"); ok {
		t.Fatal("empty list should allow nothing")
	}
}
