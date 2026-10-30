package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLoginResponse_UnmarshalExpirations(t *testing.T) {
	body := `{"accessTokenExpiration":"2026-10-23T22:35:43Z","refreshTokenExpiration":"2026-10-24T22:25:43Z"}`

	var resp LoginResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	wantAccess := time.Date(2026, 10, 23, 22, 35, 43, 0, time.UTC)
	wantRefresh := time.Date(2026, 10, 24, 22, 25, 43, 0, time.UTC)
	if !resp.AccessTokenExpiration.Equal(wantAccess) {
		t.Errorf("AccessTokenExpiration = %v, want %v", resp.AccessTokenExpiration, wantAccess)
	}
	if !resp.RefreshTokenExpiration.Equal(wantRefresh) {
		t.Errorf("RefreshTokenExpiration = %v, want %v", resp.RefreshTokenExpiration, wantRefresh)
	}
}

func TestLoginResponse_UnmarshalMalformedExpiration(t *testing.T) {
	body := `{"accessTokenExpiration":"not-a-time","refreshTokenExpiration":"2026-10-24T22:25:43Z"}`

	var resp LoginResponse
	if err := json.Unmarshal([]byte(body), &resp); err == nil {
		t.Error("Unmarshal() error = nil, want error for malformed timestamp")
	}
}

func TestLoginResponse_TokensNotReadFromBody(t *testing.T) {
	// Tokens arrive as cookies only; a body that happens to carry them must not populate them.
	body := `{"okapiToken":"body-token","refreshToken":"body-refresh","accessTokenExpiration":"2026-10-23T22:35:43Z"}`

	var resp LoginResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if resp.OkapiToken != "" || resp.RefreshToken != "" {
		t.Errorf("tokens populated from body: OkapiToken=%q RefreshToken=%q", resp.OkapiToken, resp.RefreshToken)
	}
}
