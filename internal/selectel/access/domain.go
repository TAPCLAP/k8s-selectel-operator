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
	"fmt"
	"regexp"
	"strings"
)

// patternError marks an allowedDomains entry that cannot be compiled.
type patternError struct {
	pattern string
	err     error
}

func (e *patternError) Error() string {
	return fmt.Sprintf("allowedDomains pattern %q is invalid: %s", e.pattern, e.err.Error())
}

func (e *patternError) Unwrap() error { return e.err }

// IsPatternError reports whether err is a bad allowedDomains pattern.
func IsPatternError(err error) bool {
	_, ok := err.(*patternError)
	return ok
}

// AllowsDomains reports whether every name matches a pattern.
// An empty pattern list allows every name.
func AllowsDomains(patterns, names []string) error {
	if len(patterns) == 0 || len(names) == 0 {
		return nil
	}
	var invalid error
	for _, name := range names {
		matched := false
		for _, pattern := range patterns {
			ok, err := matchDomain(pattern, name)
			if err != nil {
				if invalid == nil {
					invalid = err
				}
				continue
			}
			if ok {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if invalid != nil {
			return invalid
		}
		return fmt.Errorf("domain %q is not allowed", name)
	}
	return nil
}

func matchDomain(pattern, name string) (bool, error) {
	if len(pattern) >= 2 && strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") {
		expression := pattern[1 : len(pattern)-1]
		re, err := regexp.Compile(`\A(?:` + expression + `)\z`)
		if err != nil {
			return false, &patternError{pattern: pattern, err: err}
		}
		return re.MatchString(name), nil
	}
	return matchGlob(strings.ToLower(pattern), strings.ToLower(name)), nil
}

// matchGlob matches a domain glob. '*' is one label, '**' crosses dots, '?' is one character inside a label.
func matchGlob(pattern, name string) bool {
	for {
		if pattern == "" {
			return name == ""
		}
		if strings.HasPrefix(pattern, "**") {
			rest := pattern[2:]
			for i := 0; i <= len(name); i++ {
				if matchGlob(rest, name[i:]) {
					return true
				}
			}
			return false
		}
		switch pattern[0] {
		case '*':
			rest := pattern[1:]
			for i := 0; i <= len(name); i++ {
				if i > 0 && name[i-1] == '.' {
					break
				}
				if matchGlob(rest, name[i:]) {
					return true
				}
			}
			return false
		case '?':
			if name == "" || name[0] == '.' {
				return false
			}
			pattern = pattern[1:]
			name = name[1:]
		default:
			if name == "" || pattern[0] != name[0] {
				return false
			}
			pattern = pattern[1:]
			name = name[1:]
		}
	}
}
