package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// listResponse is the wrapper for LibreGraph list endpoints.
type listResponse[T any] struct {
	Value []T `json:"value"`
}

// GetJSON performs a GET request and decodes the JSON response into T.
func GetJSON[T any](ctx context.Context, c *Client, path string, query url.Values) (T, error) {
	var zero T
	fullPath := path
	if len(query) > 0 {
		fullPath = path + "?" + query.Encode()
	}
	req, err := c.NewRequest(ctx, http.MethodGet, fullPath, nil)
	if err != nil {
		return zero, err
	}
	resp, err := c.DoJSON(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return zero, fmt.Errorf("decoding response: %w", err)
	}
	return result, nil
}

// ListJSON performs a GET request and unwraps a {"value": [...]} response.
func ListJSON[T any](ctx context.Context, c *Client, path string, query url.Values) ([]T, error) {
	fullPath := path
	if len(query) > 0 {
		fullPath = path + "?" + query.Encode()
	}
	req, err := c.NewRequest(ctx, http.MethodGet, fullPath, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.DoJSON(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result listResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding list response: %w", err)
	}
	return result.Value, nil
}

// PostJSON performs a POST request with a JSON body and decodes the response.
func PostJSON[T any](ctx context.Context, c *Client, path string, body any) (T, error) {
	var zero T
	data, err := json.Marshal(body)
	if err != nil {
		return zero, fmt.Errorf("marshaling request body: %w", err)
	}
	req, err := c.NewRequest(ctx, http.MethodPost, path, bytes.NewReader(data))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.DoJSON(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return zero, fmt.Errorf("decoding response: %w", err)
	}
	return result, nil
}

// PatchJSON performs a PATCH request with a JSON body and decodes the response.
func PatchJSON[T any](ctx context.Context, c *Client, path string, body any) (T, error) {
	var zero T
	data, err := json.Marshal(body)
	if err != nil {
		return zero, fmt.Errorf("marshaling request body: %w", err)
	}
	req, err := c.NewRequest(ctx, http.MethodPatch, path, bytes.NewReader(data))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.DoJSON(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return zero, fmt.Errorf("decoding response: %w", err)
	}
	return result, nil
}

// Delete performs a DELETE request.
func Delete(ctx context.Context, c *Client, path string) error {
	req, err := c.NewRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return errorFromResponse(resp)
	}
	return nil
}

// DeleteWithHeaders performs a DELETE request with additional headers.
func DeleteWithHeaders(ctx context.Context, c *Client, path string, headers map[string]string) error {
	req, err := c.NewRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return errorFromResponse(resp)
	}
	return nil
}

// PostJSONRaw performs a POST request with a JSON body and returns raw bytes.
func PostJSONRaw(ctx context.Context, c *Client, path string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request body: %w", err)
	}
	req, err := c.NewRequest(ctx, http.MethodPost, path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.DoJSON(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// GetRaw performs a GET request and returns raw bytes.
func GetRaw(ctx context.Context, c *Client, path string) ([]byte, int, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	return body, resp.StatusCode, err
}
