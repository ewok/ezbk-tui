/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ezbk

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const errCodeInvalidTokenType = 202004

var tokenTypeNames = map[int]string{
	1: "normal session",
	2: "2FA-pending",
	3: "email verification",
	4: "password reset",
	5: "MCP",
	6: "OAuth2 callback (unverified)",
	7: "OAuth2 callback",
	8: "API",
}

// tokenType extracts the ezBookkeeping token type from the JWT payload (without verifying it).
func tokenType(token string) (int, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return 0, false
	}
	var claims struct {
		Type int `json:"type"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, false
	}
	return claims.Type, true
}

// explainAuthError adds a hint when the server rejects the token type.
// apiTokensEnabled is nil when the server setting could not be determined.
func explainAuthError(err error, token string, apiTokensEnabled *bool) error {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != errCodeInvalidTokenType {
		return err
	}
	if apiTokensEnabled != nil && !*apiTokensEnabled {
		return fmt.Errorf("API tokens are disabled on this ezBookkeeping server: set enable_api_token = true in [security] "+
			"(or EBK_SECURITY_ENABLE_API_TOKEN=true), restart it, then generate an API token in User Settings -> Security: %w", err)
	}
	hint := "an API token is required (User Settings -> Security -> Generate Token, type API; server needs enable_api_token = true)"
	if t, ok := tokenType(token); ok {
		name, known := tokenTypeNames[t]
		if !known {
			name = fmt.Sprintf("type %d", t)
		}
		if t == 8 {
			hint = "API tokens seem to be disabled on the server (set enable_api_token = true)"
		}
		return fmt.Errorf("the configured token is a %s token: %s: %w", name, hint, err)
	}
	return fmt.Errorf("%s: %w", hint, err)
}
