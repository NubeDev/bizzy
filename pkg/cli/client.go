package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client is an HTTP client that injects auth from config.
type Client struct {
	Server string
	Token  string
}

// NewClient creates a client from the saved config.
func NewClient() (*Client, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Client{Server: cfg.Server, Token: cfg.Token}, nil
}

// NewClientFrom creates a client from explicit values (for flag overrides).
func NewClientFrom(server, token string) *Client {
	return &Client{Server: strings.TrimRight(server, "/"), Token: token}
}

// Do executes an HTTP request and returns the response body as raw bytes.
func (c *Client) Do(method, path string, body any) (int, []byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	url := c.Server + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}

	return resp.StatusCode, data, nil
}

// DoJSON executes a request and unmarshals the response into result.
func (c *Client) DoJSON(method, path string, body any, result any) (int, error) {
	status, data, err := c.Do(method, path, body)
	if err != nil {
		return status, err
	}
	if result != nil && len(data) > 0 {
		if err := json.Unmarshal(data, result); err != nil {
			return status, fmt.Errorf("unmarshal: %w\nbody: %s", err, string(data))
		}
	}
	return status, nil
}
