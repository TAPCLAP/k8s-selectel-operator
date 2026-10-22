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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultAuthURL is the Selectel Keystone identity root.
	DefaultAuthURL = "https://cloud.api.selcloud.ru/identity/v3"

	tokenTTL = 24 * time.Hour
)

// Credentials are the Keystone password identity for one project.
type Credentials struct {
	AccountID   string
	Username    string
	Password    string
	ProjectName string
}

// Client issues project-scoped IAM tokens.
type Client struct {
	HTTP *http.Client
}

// NewClient returns a token client. A nil HTTP client uses a 30s timeout.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{HTTP: httpClient}
}

type tokenResponse struct {
	Token struct {
		ExpiresAt string `json:"expires_at"`
	} `json:"token"`
}

// IssueToken requests a project-scoped token. The token is returned to the caller
// and must not be logged.
func (c *Client) IssueToken(ctx context.Context, authURL string, creds Credentials) (string, time.Time, error) {
	if authURL == "" {
		authURL = DefaultAuthURL
	}
	payload := map[string]any{
		"auth": map[string]any{
			"identity": map[string]any{
				"methods": []string{"password"},
				"password": map[string]any{
					"user": map[string]any{
						"name":     creds.Username,
						"password": creds.Password,
						"domain":   map[string]any{"name": creds.AccountID},
					},
				},
			},
			"scope": map[string]any{
				"project": map[string]any{
					"name":   creds.ProjectName,
					"domain": map[string]any{"name": creds.AccountID},
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", time.Time{}, err
	}
	endpoint := strings.TrimRight(authURL, "/") + "/auth/tokens"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", time.Time{}, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", time.Time{}, fmt.Errorf("iam token request failed with status %d", resp.StatusCode)
	}
	token := resp.Header.Get("X-Subject-Token")
	if token == "" {
		return "", time.Time{}, fmt.Errorf("iam token response did not include X-Subject-Token")
	}
	var parsed tokenResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", time.Time{}, fmt.Errorf("decode iam token response: %w", err)
	}
	expires := time.Now().Add(tokenTTL)
	if parsed.Token.ExpiresAt != "" {
		parsedExpiry, err := time.Parse(time.RFC3339, parsed.Token.ExpiresAt)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("parse token expiry: %w", err)
		}
		expires = parsedExpiry
	}
	return token, expires, nil
}
