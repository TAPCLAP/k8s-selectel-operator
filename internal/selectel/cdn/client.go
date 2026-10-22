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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/tapclap/k8s-selectel-operator/internal/selectel/cdn/cdnapi"
)

const DefaultBaseURL = "https://api.selectel.ru/cdn/v3"

var (
	// ErrUnauthorized is a 401 from the CDN API. The caller should drop the IAM token and retry once.
	ErrUnauthorized = errors.New("cdn api unauthorized")
	// ErrNotFound is a 404 from the CDN API.
	ErrNotFound = errors.New("cdn api not found")
)

// RateLimitError is a 429. RetryAfter is 30s when the API omits Retry-After.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("cdn api rate limited, retry after %s", e.RetryAfter)
}

// Mutation is the create, put, and patch response body.
type Mutation struct {
	Status      string
	ResourceID  string
	CDNDomain   string
	TaskID      string
	Description string
	Message     string
}

// Status is the Selectel resource status.
type Status struct {
	Status  string
	Message string
}

// Client calls the Selectel CDN API with a generated OpenAPI client.
type Client struct {
	api *cdnapi.Client
}

// New returns a CDN client. baseURL is the API root, including /cdn/v3.
func New(baseURL string, httpClient *http.Client) (*Client, error) {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	api, err := cdnapi.NewClient(baseURL, cdnapi.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}
	return &Client{api: api}, nil
}

func withToken(token string) cdnapi.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("X-Auth-Token", token)
		return nil
	}
}

// Create creates a CDN resource.
func (c *Client) Create(ctx context.Context, token string, body []byte) (Mutation, error) {
	resp, err := c.api.CreateResourceWithBody(ctx, "application/json", bytes.NewReader(body), withToken(token))
	if err != nil {
		return Mutation{}, err
	}
	return decodeMutation(resp)
}

// Put replaces a CDN resource configuration.
func (c *Client) Put(ctx context.Context, token, id string, body []byte) (Mutation, error) {
	uid, err := parseID(id)
	if err != nil {
		return Mutation{}, err
	}
	resp, err := c.api.UpdateResourcePutWithBody(ctx, uid, "application/json", bytes.NewReader(body), withToken(token))
	if err != nil {
		return Mutation{}, err
	}
	return decodeMutation(resp)
}

// Patch updates first-level fields. The controller uses it to set active=false.
func (c *Client) Patch(ctx context.Context, token, id string, body []byte) (Mutation, error) {
	uid, err := parseID(id)
	if err != nil {
		return Mutation{}, err
	}
	resp, err := c.api.UpdateResourcePatchWithBody(ctx, uid, "application/json", bytes.NewReader(body), withToken(token))
	if err != nil {
		return Mutation{}, err
	}
	return decodeMutation(resp)
}

// Get returns the resource JSON object.
func (c *Client) Get(ctx context.Context, token, id string) (map[string]any, error) {
	uid, err := parseID(id)
	if err != nil {
		return nil, err
	}
	resp, err := c.api.GetResourceById(ctx, uid, withToken(token))
	if err != nil {
		return nil, err
	}
	raw, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	if err := classify(resp, raw); err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode cdn resource: %w", err)
	}
	return doc, nil
}

// List returns active and inactive resources. The API defaults the active filter to true,
// so both values are requested.
func (c *Client) List(ctx context.Context, token string) ([]map[string]any, error) {
	active, err := c.list(ctx, token, true)
	if err != nil {
		return nil, err
	}
	inactive, err := c.list(ctx, token, false)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	out := make([]map[string]any, 0, len(active)+len(inactive))
	for _, item := range append(active, inactive...) {
		id, _ := item["id"].(string)
		if id != "" {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
		}
		out = append(out, item)
	}
	return out, nil
}

func (c *Client) list(ctx context.Context, token string, active bool) ([]map[string]any, error) {
	resp, err := c.api.ListResources(ctx, &cdnapi.ListResourcesParams{Active: &active}, withToken(token))
	if err != nil {
		return nil, err
	}
	raw, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	if err := classify(resp, raw); err != nil {
		return nil, err
	}
	var items []map[string]any
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decode cdn resource list: %w", err)
	}
	return items, nil
}

// Status returns the configuration status of one resource.
func (c *Client) Status(ctx context.Context, token, id string) (Status, error) {
	uid, err := parseID(id)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.api.GetResourceStatus(ctx, uid, withToken(token))
	if err != nil {
		return Status{}, err
	}
	raw, err := readLimited(resp)
	if err != nil {
		return Status{}, err
	}
	if err := classify(resp, raw); err != nil {
		return Status{}, err
	}
	var parsed cdnapi.ModelsResourceStatus
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Status{}, fmt.Errorf("decode cdn status: %w", err)
	}
	out := Status{}
	if parsed.Status != nil {
		out.Status = string(*parsed.Status)
	}
	if parsed.Message != nil {
		out.Message = string(*parsed.Message)
	}
	return out, nil
}

func parseID(id string) (openapi_types.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse resource id: %w", err)
	}
	return parsed, nil
}

func decodeMutation(resp *http.Response) (Mutation, error) {
	raw, err := readLimited(resp)
	if err != nil {
		return Mutation{}, err
	}
	if err := classify(resp, raw); err != nil {
		return Mutation{}, err
	}
	var parsed cdnapi.ModelsCreateUpdateResponse
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return Mutation{}, fmt.Errorf("decode cdn mutation: %w", err)
		}
	}
	out := Mutation{}
	if parsed.Status != nil {
		out.Status = string(*parsed.Status)
	}
	if parsed.ResourceId != nil {
		out.ResourceID = *parsed.ResourceId
	}
	if parsed.CdnDomain != nil {
		out.CDNDomain = *parsed.CdnDomain
	}
	if parsed.TaskId != nil {
		out.TaskID = *parsed.TaskId
	}
	if parsed.Description != nil {
		out.Description = *parsed.Description
	}
	if parsed.Message != nil {
		out.Message = *parsed.Message
	}
	return out, nil
}

func readLimited(resp *http.Response) ([]byte, error) {
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func classify(resp *http.Response, body []byte) error {
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		wait := 30 * time.Second
		if header := resp.Header.Get("Retry-After"); header != "" {
			if seconds, err := strconv.Atoi(header); err == nil && seconds > 0 {
				wait = time.Duration(seconds) * time.Second
			}
		}
		return &RateLimitError{RetryAfter: wait}
	default:
		return fmt.Errorf("cdn api status %d: %s", resp.StatusCode, truncate(body))
	}
}

func truncate(body []byte) string {
	const max = 512
	if len(body) <= max {
		return string(body)
	}
	return string(body[:max])
}
