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

import selectelv1alpha1 "github.com/tapclap/k8s-selectel-operator/api/selectel/v1alpha1"

// Lookup returns the namespace entry. An exact name wins over "*".
// The "*" entry is not merged into an exact entry.
func Lookup(entries []selectelv1alpha1.NamespaceAccess, namespace string) (selectelv1alpha1.NamespaceAccess, bool) {
	var wildcard *selectelv1alpha1.NamespaceAccess
	for i := range entries {
		if entries[i].Name == namespace {
			return entries[i], true
		}
		if entries[i].Name == "*" && wildcard == nil {
			copy := entries[i]
			wildcard = &copy
		}
	}
	if wildcard != nil {
		return *wildcard, true
	}
	return selectelv1alpha1.NamespaceAccess{}, false
}

// AllowsCertificate reports whether the entry lists the Selectel certificate seq.
func AllowsCertificate(entry selectelv1alpha1.NamespaceAccess, id int32) bool {
	for _, cert := range entry.Certificates {
		if cert.ID == id {
			return true
		}
	}
	return false
}
