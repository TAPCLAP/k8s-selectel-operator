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
	"sync"
	"time"
)

const refreshBefore = 30 * time.Minute

type cachedToken struct {
	token    string
	expires  time.Time
	credsKey string
}

// Cache keeps project-scoped tokens in memory. The key is the Project name.
// Tokens are refreshed 30 minutes before they expire and never written to status or logs.
type Cache struct {
	client *Client
	mu     sync.Mutex
	items  map[string]cachedToken
}

// NewCache returns an empty token cache.
func NewCache(client *Client) *Cache {
	return &Cache{client: client, items: map[string]cachedToken{}}
}

// Token returns a usable token for the Project, issuing a new one when needed.
func (c *Cache) Token(ctx context.Context, projectName, authURL string, creds Credentials) (string, error) {
	key := creds.AccountID + "\n" + creds.Username + "\n" + creds.Password + "\n" + creds.ProjectName
	c.mu.Lock()
	item, ok := c.items[projectName]
	if ok && item.credsKey == key && time.Until(item.expires) > refreshBefore {
		token := item.token
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	token, expires, err := c.client.IssueToken(ctx, authURL, creds)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.items[projectName] = cachedToken{token: token, expires: expires, credsKey: key}
	c.mu.Unlock()
	return token, nil
}

// Invalidate drops the cached token so the next call issues a new one.
func (c *Cache) Invalidate(projectName string) {
	c.mu.Lock()
	delete(c.items, projectName)
	c.mu.Unlock()
}
