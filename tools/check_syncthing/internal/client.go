package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxBodyBytes bounds every REST response the plugin reads.
const maxBodyBytes = 8 << 20

// Folder is the subset of a /rest/config/folders entry the check uses.
type Folder struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Paused bool   `json:"paused"`
}

// FolderStatus is the subset of /rest/db/status the check uses.
type FolderStatus struct {
	State          string `json:"state"`
	Error          string `json:"error"`
	NeedTotalItems int64  `json:"needTotalItems"`
	NeedBytes      int64  `json:"needBytes"`
	PullErrors     int64  `json:"pullErrors"`
	Errors         int64  `json:"errors"`
}

// Client queries a Syncthing REST API. The API key only ever leaves the
// process in the X-API-Key request header.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewClient returns a client for the Syncthing instance at baseURL.
func NewClient(baseURL, apiKey string, httpClient *http.Client) Client {
	return Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, http: httpClient}
}

// Folders lists the configured folders.
func (c Client) Folders(ctx context.Context) ([]Folder, error) {
	var folders []Folder
	if err := c.get(ctx, "/rest/config/folders", nil, &folders); err != nil {
		return nil, err
	}
	return folders, nil
}

// FolderStatus returns the database status of one folder. Syncthing
// documents this call as expensive, so run the check every few minutes.
func (c Client) FolderStatus(ctx context.Context, id string) (FolderStatus, error) {
	var status FolderStatus
	err := c.get(ctx, "/rest/db/status", url.Values{"folder": {id}}, &status)
	return status, err
}

// get fetches path and decodes the JSON body into out. Errors carry the
// path and HTTP status only, never the key or the response body.
func (c Client) get(ctx context.Context, path string, query url.Values, out any) error {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", path, err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, redactURLError(err))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", path, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// redactURLError drops the request URL from a *url.Error so the summary
// line stays short; the cause (DNS, refused, timeout) is kept.
func redactURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}
