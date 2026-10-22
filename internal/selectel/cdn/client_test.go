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

package cdn

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const resourceID = "6d19b7a2-a493-48fa-a9ec-aea7ba9ffa28"

func TestCreateGetPutStatus(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth-Token") != "tok" {
			t.Errorf("missing token")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/resources":
			_, _ = w.Write([]byte(`{"status":"accept","resource_id":"` + resourceID + `","cdn_domain":"` + resourceID + `.selcdn.net","task_id":"task-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/resources/"+resourceID:
			_, _ = w.Write([]byte(`{"id":"` + resourceID + `","name":"static"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/resources/"+resourceID:
			_, _ = w.Write([]byte(`{"status":"accept","task_id":"task-2"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/resources/"+resourceID+"/status":
			_, _ = w.Write([]byte(`{"status":"Completed","message":"configuration processing","resource":"` + resourceID + `"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/resources":
			_, _ = w.Write([]byte(`[{"id":"` + resourceID + `","name":"static"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	created, err := client.Create(ctx, "tok", []byte(`{"name":"static"}`))
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != "accept" || created.ResourceID != resourceID || created.TaskID != "task-1" {
		t.Fatalf("create = %+v", created)
	}
	got, err := client.Get(ctx, "tok", resourceID)
	if err != nil || got["name"] != "static" {
		t.Fatalf("get = %v %v", got, err)
	}
	updated, err := client.Put(ctx, "tok", resourceID, []byte(`{"name":"static"}`))
	if err != nil || updated.Status != "accept" {
		t.Fatalf("put = %+v %v", updated, err)
	}
	status, err := client.Status(ctx, "tok", resourceID)
	if err != nil || status.Status != "Completed" {
		t.Fatalf("status = %+v %v", status, err)
	}
	listed, err := client.List(ctx, "tok")
	if err != nil || len(listed) != 1 {
		t.Fatalf("list = %v %v", listed, err)
	}
}

func TestCreateErrorBody(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"error","message":"Json invalid","description":"Key incorrect or not found ( origin )"}`))
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Create(context.Background(), "tok", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "error" || got.Message != "Json invalid" || got.Description == "" {
		t.Fatalf("mutation = %+v", got)
	}
}

func TestUnauthorizedNotFoundAndRateLimit(t *testing.T) {
	t.Parallel()
	newServer := func(status int, header string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if header != "" {
				w.Header().Set("Retry-After", header)
			}
			w.WriteHeader(status)
		}))
	}
	unauthorizedSrv := newServer(http.StatusUnauthorized, "")
	t.Cleanup(unauthorizedSrv.Close)
	missingSrv := newServer(http.StatusNotFound, "")
	t.Cleanup(missingSrv.Close)
	limitedSrv := newServer(http.StatusTooManyRequests, "7")
	t.Cleanup(limitedSrv.Close)

	unauthorized, err := New(unauthorizedSrv.URL, unauthorizedSrv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unauthorized.Create(context.Background(), "tok", []byte(`{"name":"a"}`)); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v", err)
	}

	missing, err := New(missingSrv.URL, missingSrv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := missing.Get(context.Background(), "tok", resourceID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}

	limited, err := New(limitedSrv.URL, limitedSrv.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = limited.Status(context.Background(), "tok", resourceID)
	var rate *RateLimitError
	if !errors.As(err, &rate) || rate.RetryAfter.Seconds() != 7 {
		t.Fatalf("err = %v", err)
	}
}
