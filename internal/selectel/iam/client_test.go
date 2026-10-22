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

package iam

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestIssueToken(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/auth/tokens" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "issued-token") {
			t.Errorf("response token leaked into the request")
		}
		w.Header().Set("X-Subject-Token", "issued-token")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":{"expires_at":"2099-01-01T00:00:00Z"}}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.Client())
	token, expires, err := client.IssueToken(context.Background(), server.URL, Credentials{
		AccountID:   "123",
		Username:    "user",
		Password:    "secret-pass",
		ProjectName: "prod",
	})
	if err != nil {
		t.Fatal(err)
	}
	if token != "issued-token" {
		t.Fatalf("token = %q", token)
	}
	if expires.Year() != 2099 {
		t.Fatalf("expires = %s", expires)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d", hits.Load())
	}
}

func TestCacheReusesTokenUntilInvalidated(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("X-Subject-Token", time.Now().Format(time.RFC3339Nano))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":{"expires_at":"` + time.Now().Add(2*time.Hour).UTC().Format(time.RFC3339) + `"}}`))
	}))
	t.Cleanup(server.Close)

	cache := NewCache(NewClient(server.Client()))
	creds := Credentials{AccountID: "1", Username: "u", Password: "p", ProjectName: "proj"}
	ctx := context.Background()
	first, err := cache.Token(ctx, "production", server.URL, creds)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Token(ctx, "production", server.URL, creds)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("cache miss: %q %q", first, second)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d", hits.Load())
	}
	cache.Invalidate("production")
	third, err := cache.Token(ctx, "production", server.URL, creds)
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Fatal("expected a new token after invalidation")
	}
	if hits.Load() != 2 {
		t.Fatalf("hits = %d", hits.Load())
	}
}

func TestCacheRefreshesNearExpiry(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("X-Subject-Token", "short")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":{"expires_at":"` + time.Now().Add(10*time.Minute).UTC().Format(time.RFC3339) + `"}}`))
	}))
	t.Cleanup(server.Close)

	cache := NewCache(NewClient(server.Client()))
	creds := Credentials{AccountID: "1", Username: "u", Password: "p", ProjectName: "proj"}
	if _, err := cache.Token(context.Background(), "production", server.URL, creds); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Token(context.Background(), "production", server.URL, creds); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 {
		t.Fatalf("hits = %d, want a refresh inside the 30m window", hits.Load())
	}
}
