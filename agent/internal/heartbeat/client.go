package heartbeat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const heartbeatPath = "/api/v1/heartbeat"

// Client sends worker node heartbeat snapshots to Coordinator.
type Client struct {
	endpoint   string
	httpClient *http.Client
}

// NewClient creates a heartbeat HTTP client.
func NewClient(endpoint string, timeout time.Duration) (*Client, error) {
	endpoint = strings.TrimSpace(endpoint)

	if endpoint == "" {
		return nil, fmt.Errorf("coordinator endpoint cannot be empty")
	}

	if timeout <= 0 {
		return nil, fmt.Errorf("HTTP timeout must be greater than zero")
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse coordinator endpoint: %w", err)
	}

	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("invalid coordinator endpoint: %s", endpoint)
	}

	u.Path = strings.TrimRight(u.Path, "/") + heartbeatPath
	u.RawPath = ""

	return &Client{
		endpoint: u.String(),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// Send sends a snapshot to Coordinator.
func (c *Client) Send(ctx context.Context, snapshot any) error {
	if c == nil {
		return fmt.Errorf("heartbeat client cannot be nil")
	}

	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}

	if snapshot == nil {
		return fmt.Errorf("heartbeat snapshot cannot be nil")
	}

	body, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal heartbeat snapshot: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("create heartbeat request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send heartbeat request: %w", err)
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("coordinator returned HTTP %d", resp.StatusCode)
	}

	return nil
}
