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

import "testing"

func TestAllowsDomains(t *testing.T) {
	t.Parallel()
	patterns := []string{"debug-cdn.g.tapclap.com", "*.tapclap.com", "**.example.com", `/.+\.cdn\.test/`}
	allowed := []string{
		"debug-cdn.g.tapclap.com",
		"CDN.tapclap.com",
		"a.b.example.com",
		"img.cdn.test",
	}
	if err := AllowsDomains(patterns, allowed); err != nil {
		t.Fatal(err)
	}
	if err := AllowsDomains(nil, []string{"anywhere.example"}); err != nil {
		t.Fatal(err)
	}
	if err := AllowsDomains(patterns, []string{"a.b.tapclap.com"}); err == nil || IsPatternError(err) {
		t.Fatalf("expected a rejected domain, got %v", err)
	}
	if err := AllowsDomains([]string{"/[unclosed/"}, []string{"a.example"}); err == nil || !IsPatternError(err) {
		t.Fatalf("expected an invalid pattern, got %v", err)
	}
}
