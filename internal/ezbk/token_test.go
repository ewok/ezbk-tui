/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ezbk

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeJWT(payload string) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"HS256"}`)) + "." + enc([]byte(payload)) + ".sig"
}

func TestTokenType(t *testing.T) {
	tests := []struct {
		name   string
		token  string
		want   int
		wantOK bool
	}{
		{"mcp", fakeJWT(`{"type":5}`), 5, true},
		{"api", fakeJWT(`{"type":8}`), 8, true},
		{"not jwt", "abc", 0, false},
		{"bad payload", "a.!!!.c", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tokenType(tt.token)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("tokenType = %d, %v", got, ok)
			}
		})
	}
}

func TestExplainAuthError(t *testing.T) {
	invalid := &APIError{Code: errCodeInvalidTokenType, Message: "current token type is invalid"}
	off, on := false, true
	tests := []struct {
		name    string
		err     error
		token   string
		enabled *bool
		want    string
	}{
		{"mcp token", invalid, fakeJWT(`{"type":5}`), nil, "is a MCP token"},
		{"mcp token, server disabled", invalid, fakeJWT(`{"type":5}`), &off, "API tokens are disabled on this ezBookkeeping server"},
		{"mcp token, server enabled", invalid, fakeJWT(`{"type":5}`), &on, "is a MCP token"},
		{"api token disabled", invalid, fakeJWT(`{"type":8}`), nil, "enable_api_token"},
		{"opaque token", invalid, "xyz", nil, "an API token is required"},
		{"other error untouched", errors.New("boom"), "xyz", &off, "boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := explainAuthError(fmt.Errorf("wrap: %w", tt.err), tt.token, tt.enabled)
			if !strings.Contains(got.Error(), tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if !errors.Is(got, tt.err) {
				t.Error("original error must be wrapped")
			}
		})
	}
}

func TestApiTokensEnabled(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"disabled", "_['a']=1;\n_['t']=0;\n", "false"},
		{"enabled", "_['t']=1;", "true"},
		{"unknown", "nothing", "nil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/server_settings.js" {
					_, _ = io.WriteString(w, tt.body)
				}
			}))
			defer srv.Close()
			c, err := NewClient(Config{ApiUrl: srv.URL})
			if err != nil {
				t.Fatal(err)
			}
			got := "nil"
			if v := c.apiTokensEnabled(); v != nil {
				got = fmt.Sprint(*v)
			}
			if got != tt.want {
				t.Errorf("apiTokensEnabled = %s, want %s", got, tt.want)
			}
		})
	}
}
