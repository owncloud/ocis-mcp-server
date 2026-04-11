package client

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAPIErrorString(t *testing.T) {
	err := &APIError{StatusCode: 404, Message: "resource not found"}
	got := err.Error()
	if !strings.Contains(got, "404") || !strings.Contains(got, "resource not found") {
		t.Errorf("Error() = %q, expected to contain 404 and message", got)
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"404 API error", &APIError{StatusCode: 404}, true},
		{"403 API error", &APIError{StatusCode: 403}, false},
		{"generic error", fmt.Errorf("generic"), false},
		{"nil error", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsForbidden(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"403 API error", &APIError{StatusCode: 403}, true},
		{"404 API error", &APIError{StatusCode: 404}, false},
		{"generic error", fmt.Errorf("generic"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsForbidden(tt.err); got != tt.want {
				t.Errorf("IsForbidden() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsConflict(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"409 API error", &APIError{StatusCode: 409}, true},
		{"404 API error", &APIError{StatusCode: 404}, false},
		{"generic error", fmt.Errorf("generic"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConflict(tt.err); got != tt.want {
				t.Errorf("IsConflict() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorFromResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantMsg    string
	}{
		{
			name:       "JSON error.message",
			statusCode: 400,
			body:       `{"error":{"message":"invalid field","code":"invalidField"}}`,
			wantMsg:    "invalid field",
		},
		{
			name:       "OCS meta.message",
			statusCode: 400,
			body:       `{"ocs":{"meta":{"message":"OCS error message","statuscode":400}}}`,
			wantMsg:    "OCS error message",
		},
		{
			name:       "fallback to status message",
			statusCode: 502,
			body:       `not json`,
			wantMsg:    "bad gateway",
		},
		{
			name:       "empty body uses status message",
			statusCode: 503,
			body:       "",
			wantMsg:    "service unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}
			err := errorFromResponse(resp)
			apiErr, ok := err.(*APIError)
			if !ok {
				t.Fatalf("expected *APIError, got %T", err)
			}
			if apiErr.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.statusCode)
			}
			if !strings.Contains(apiErr.Message, tt.wantMsg) {
				t.Errorf("Message = %q, want it to contain %q", apiErr.Message, tt.wantMsg)
			}
		})
	}
}

func TestHttpStatusMessage(t *testing.T) {
	tests := []struct {
		code    int
		wantMsg string
	}{
		{400, "bad request"},
		{401, "authentication failed"},
		{403, "insufficient permissions"},
		{404, "not found"},
		{409, "conflict"},
		{500, "internal server error"},
		{502, "bad gateway"},
		{503, "service unavailable"},
		{418, "unexpected status 418"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.code), func(t *testing.T) {
			got := httpStatusMessage(tt.code)
			if !strings.Contains(got, tt.wantMsg) {
				t.Errorf("httpStatusMessage(%d) = %q, want to contain %q", tt.code, got, tt.wantMsg)
			}
		})
	}
}
