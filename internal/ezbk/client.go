/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ezbk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Config holds connection settings.
type Config struct {
	ApiUrl         string
	Token          string
	Timezone       string
	TimeoutSeconds int
}

// APIError is an error returned by the ezBookkeeping API envelope.
type APIError struct {
	Code    int
	Message string
	Path    string
	Status  int
}

func (e *APIError) Error() string {
	if e.Code != 0 {
		return fmt.Sprintf("ezBookkeeping error %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("ezBookkeeping HTTP %d: %s", e.Status, e.Message)
}

type envelope struct {
	Success      bool            `json:"success"`
	Result       json.RawMessage `json:"result"`
	ErrorCode    int             `json:"errorCode"`
	ErrorMessage string          `json:"errorMessage"`
	Path         string          `json:"path"`
}

// Client is a thin HTTP client for the ezBookkeeping API.
type Client struct {
	baseURL  string
	token    string
	location *time.Location
	http     *http.Client
	timeout  time.Duration
}

// NewClient builds a client; the base URL may or may not include /api/v1.
func NewClient(cfg Config) (*Client, error) {
	base, err := normalizeBaseURL(cfg.ApiUrl)
	if err != nil {
		return nil, err
	}
	loc := time.Local
	if cfg.Timezone != "" {
		loc, err = time.LoadLocation(cfg.Timezone)
		if err != nil {
			return nil, fmt.Errorf("invalid timezone %q: %w", cfg.Timezone, err)
		}
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		baseURL:  base,
		token:    cfg.Token,
		location: loc,
		http:     &http.Client{Timeout: timeout},
		timeout:  timeout,
	}, nil
}

func normalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid ezBookkeeping URL %q", raw)
	}
	if !strings.HasSuffix(u.Path, "/api/v1") {
		u.Path = strings.TrimSuffix(u.Path, "/api") + "/api/v1"
	}
	return u.String(), nil
}

// Location returns the timezone used for requests and period calculations.
func (c *Client) Location() *time.Location {
	return c.location
}

func (c *Client) get(path string, params url.Values, out any) error {
	endpoint := c.baseURL + "/" + path
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}
	return c.do(http.MethodGet, endpoint, nil, out)
}

func (c *Client) post(path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode request for %s: %w", path, err)
	}
	return c.do(http.MethodPost, c.baseURL+"/"+path, data, out)
}

func (c *Client) do(method, endpoint string, body []byte, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Timezone-Name", c.location.String())
	_, offset := time.Now().In(c.location).Zone()
	req.Header.Set("X-Timezone-Offset", strconv.Itoa(offset/60))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	zap.L().Debug("ezbk request", zap.String("method", method), zap.String("url", endpoint))

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s failed: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return &APIError{Status: resp.StatusCode, Message: truncate(string(raw), 200)}
	}
	if !env.Success {
		msg := env.ErrorMessage
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return &APIError{Code: env.ErrorCode, Message: msg, Path: env.Path, Status: resp.StatusCode}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(env.Result, out); err != nil {
		return fmt.Errorf("failed to decode result of %s: %w", endpoint, err)
	}
	return nil
}

// apiTokensEnabled reads the public server_settings.js ("t" = enable_api_token).
// It returns nil when the setting cannot be determined.
func (c *Client) apiTokensEnabled() *bool {
	root := strings.TrimSuffix(c.baseURL, "/api/v1")
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, root+"/server_settings.js", nil)
	if err != nil {
		return nil
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil
	}
	s := string(body)
	for _, v := range []bool{true, false} {
		flag := "0"
		if v {
			flag = "1"
		}
		if strings.Contains(s, "_['t']="+flag+";") {
			return &v
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
