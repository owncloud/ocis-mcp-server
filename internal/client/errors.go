package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// APIError represents an error response from the oCIS API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("oCIS API error (HTTP %d): %s", e.StatusCode, e.Message)
}

// IsNotFound returns true if the error is a 404.
func IsNotFound(err error) bool {
	if e, ok := err.(*APIError); ok {
		return e.StatusCode == 404
	}
	return false
}

// IsForbidden returns true if the error is a 403.
func IsForbidden(err error) bool {
	if e, ok := err.(*APIError); ok {
		return e.StatusCode == 403
	}
	return false
}

// IsConflict returns true if the error is a 409.
func IsConflict(err error) bool {
	if e, ok := err.(*APIError); ok {
		return e.StatusCode == 409
	}
	return false
}

func errorFromResponse(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	msg := httpStatusMessage(resp.StatusCode)

	// Try to extract error message from JSON body
	var jsonErr struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &jsonErr) == nil && jsonErr.Error.Message != "" {
		msg = jsonErr.Error.Message
	}

	// Try OCS error format
	var ocsErr struct {
		OCS struct {
			Meta struct {
				Message    string `json:"message"`
				StatusCode int    `json:"statuscode"`
			} `json:"meta"`
		} `json:"ocs"`
	}
	if json.Unmarshal(body, &ocsErr) == nil && ocsErr.OCS.Meta.Message != "" {
		msg = ocsErr.OCS.Meta.Message
	}

	return &APIError{
		StatusCode: resp.StatusCode,
		Message:    msg,
	}
}

func httpStatusMessage(code int) string {
	switch code {
	case 400:
		return "bad request"
	case 401:
		return "authentication failed — check your OIDC token or app token"
	case 403:
		return "insufficient permissions"
	case 404:
		return "resource not found"
	case 409:
		return "conflict — resource already exists or version mismatch"
	case 500:
		return "internal server error"
	case 502:
		return "bad gateway — oCIS may be unavailable"
	case 503:
		return "service unavailable"
	default:
		return fmt.Sprintf("unexpected status %d", code)
	}
}
