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
	"sort"

	cdnv1alpha1 "github.com/tapclap/k8s-selectel-operator/api/cdn.selectel/v1alpha1"
)

const (
	maxDriftEntries = 32
	maxDriftBytes   = 256
)

// Desired builds the Selectel resource JSON from the CR. Operator-only fields are removed.
// certificate.id is sent as the API integer. An empty name becomes metadata.name.
func Desired(resource *cdnv1alpha1.Resource) (map[string]any, error) {
	raw, err := json.Marshal(resource.Spec)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	delete(doc, "projectRef")
	delete(doc, "deletionPolicy")
	delete(doc, "managementPolicy")
	delete(doc, "resourceID")
	delete(doc, "certificate")
	if resource.Spec.Certificate != nil && resource.Spec.Certificate.ID != nil {
		doc["certificate"] = *resource.Spec.Certificate.ID
	}
	name, _ := doc["name"].(string)
	if name == "" {
		doc["name"] = resource.Name
	}
	return doc, nil
}

// Matches reports whether every field in desired is present and equal in actual.
// Extra fields returned by Selectel, including defaults the spec did not set, are ignored.
func Matches(desired, actual map[string]any) (bool, error) {
	left, err := canonical(desired)
	if err != nil {
		return false, err
	}
	right, err := canonical(actual)
	if err != nil {
		return false, err
	}
	desiredMap, _ := left.(map[string]any)
	actualMap, _ := right.(map[string]any)
	ignoreAssignedDomain(desiredMap, actualMap)
	return equalValue(desiredMap, actualMap), nil
}

// Diff lists desired fields that differ from actual. Extra API keys are ignored.
// The returned slice is capped; total is the full number of differences.
func Diff(desired, actual map[string]any) ([]cdnv1alpha1.FieldDrift, int, error) {
	left, err := canonical(desired)
	if err != nil {
		return nil, 0, err
	}
	right, err := canonical(actual)
	if err != nil {
		return nil, 0, err
	}
	desiredMap, _ := left.(map[string]any)
	actualMap, _ := right.(map[string]any)
	ignoreAssignedDomain(desiredMap, actualMap)
	var found []cdnv1alpha1.FieldDrift
	collectDrift("", desiredMap, actualMap, &found)
	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	total := len(found)
	if total > maxDriftEntries {
		found = found[:maxDriftEntries]
	}
	return found, total, nil
}

func collectDrift(path string, desired, actual any, out *[]cdnv1alpha1.FieldDrift) {
	if equalValue(desired, actual) {
		return
	}
	desiredMap, desiredIsMap := desired.(map[string]any)
	if desiredIsMap {
		actualMap, ok := actual.(map[string]any)
		if !ok {
			*out = append(*out, driftEntry(path, desired, actual))
			return
		}
		keys := make([]string, 0, len(desiredMap))
		for key := range desiredMap {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			next := key
			if path != "" {
				next = path + "." + key
			}
			collectDrift(next, desiredMap[key], actualMap[key], out)
		}
		return
	}
	*out = append(*out, driftEntry(path, desired, actual))
}

func driftEntry(path string, desired, actual any) cdnv1alpha1.FieldDrift {
	return cdnv1alpha1.FieldDrift{
		Path:    path,
		Desired: driftValue(desired),
		Actual:  driftValue(actual),
	}
}

func driftValue(value any) string {
	if value == nil {
		return ""
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	if len(raw) <= maxDriftBytes {
		return string(raw)
	}
	return string(raw[:maxDriftBytes]) + "..."
}

// ignoreAssignedDomain drops cdn_domain from both names lists. Selectel appends
// that assigned domain to names, and the order of the remaining names does not matter.
func ignoreAssignedDomain(desired, actual map[string]any) {
	domain, _ := actual["cdn_domain"].(string)
	if domain == "" {
		return
	}
	if names, ok := desired["names"]; ok {
		desired["names"] = namesWithout(names, domain)
	}
	if names, ok := actual["names"]; ok {
		actual["names"] = namesWithout(names, domain)
	}
}

func namesWithout(value any, domain string) any {
	items, ok := value.([]any)
	if !ok {
		return value
	}
	kept := make([]string, 0, len(items))
	for _, item := range items {
		name, ok := item.(string)
		if !ok || name == domain {
			continue
		}
		kept = append(kept, name)
	}
	sort.Strings(kept)
	out := make([]any, len(kept))
	for i, name := range kept {
		out[i] = name
	}
	return out
}

func canonical(value any) (any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func equalValue(desired, actual any) bool {
	switch typed := desired.(type) {
	case map[string]any:
		actualMap, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for key, value := range typed {
			if !equalValue(value, actualMap[key]) {
				return false
			}
		}
		return true
	case []any:
		actualSlice, ok := actual.([]any)
		if !ok || len(actualSlice) != len(typed) {
			return false
		}
		for i := range typed {
			if !equalValue(typed[i], actualSlice[i]) {
				return false
			}
		}
		return true
	default:
		return desired == actual
	}
}
