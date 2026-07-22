package metubewebhook

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DefaultJobsPath is MeTube's poll-only postprocess job listing endpoint.
const DefaultJobsPath = "/api/postprocess/jobs"

// StatusSource yields the current set of MeTube postprocess jobs. It is the seam
// the Poller reads from; tests substitute a canned in-memory source.
type StatusSource interface {
	// Jobs returns a snapshot of all currently tracked postprocess jobs.
	Jobs(ctx context.Context) ([]JobStatus, error)
}

// HTTPStatusSource is a real StatusSource that polls a live MeTube instance over
// HTTP (GET {BaseURL}{Path}) and decodes the response with ParseJobs.
type HTTPStatusSource struct {
	// BaseURL is the MeTube origin, e.g. "http://metube:8081".
	BaseURL string
	// Path overrides the jobs endpoint; empty uses DefaultJobsPath.
	Path string
	// Client overrides the HTTP client; nil uses http.DefaultClient.
	Client *http.Client
}

// Jobs performs the GET and parses the body into normalized JobStatus values.
func (h *HTTPStatusSource) Jobs(ctx context.Context) ([]JobStatus, error) {
	path := h.Path
	if path == "" {
		path = DefaultJobsPath
	}
	url := strings.TrimRight(h.BaseURL, "/") + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("metubewebhook: build status request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := h.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("metubewebhook: GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("metubewebhook: read status body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("metubewebhook: MeTube %s returned HTTP %d", url, resp.StatusCode)
	}

	return ParseJobs(body)
}
