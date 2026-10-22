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

	cdnv1alpha1 "github.com/tapclap/k8s-selectel-operator/api/cdn.selectel/v1alpha1"
)

func TestColumnsCombineDomainsAndOriginHosts(t *testing.T) {
	t.Parallel()

	resource := &cdnv1alpha1.Resource{}
	resource.Status.CDNDomain = "cd791158-e469-43f3-99db-d2b0a40f462d.selcdn.net"
	resource.Spec.Names = []string{
		"debug-cdn.g.tapclap.com",
		"cd791158-e469-43f3-99db-d2b0a40f462d.selcdn.net",
		"",
	}
	resource.Spec.Origin = &cdnv1alpha1.Origin{
		Servers: map[string]cdnv1alpha1.OriginServer{
			"origin2.com": {Port: int32Ptr(443)},
			"origin1.com": {},
			"":            {},
		},
	}

	if !applyColumns(resource) {
		t.Fatal("expected the first column fill to change status")
	}
	wantDomains := []string{
		"cd791158-e469-43f3-99db-d2b0a40f462d.selcdn.net",
		"debug-cdn.g.tapclap.com",
	}
	if !sameStrings(resource.Status.CDNDomains, wantDomains) {
		t.Fatalf("cdn domains %v", resource.Status.CDNDomains)
	}
	wantOrigins := []string{"origin1.com", "origin2.com"}
	if !sameStrings(resource.Status.Origins, wantOrigins) {
		t.Fatalf("origins %v", resource.Status.Origins)
	}
	if applyColumns(resource) {
		t.Fatal("expected a repeated fill to leave status unchanged")
	}
}

func TestColumnsStayEmptyWithoutDomainsOrOrigins(t *testing.T) {
	t.Parallel()

	resource := &cdnv1alpha1.Resource{}
	if applyColumns(resource) {
		t.Fatal("expected empty columns to stay unset")
	}
	if resource.Status.CDNDomains != nil || resource.Status.Origins != nil {
		t.Fatalf("domains %v origins %v", resource.Status.CDNDomains, resource.Status.Origins)
	}
}

func int32Ptr(value int32) *int32 {
	return &value
}
