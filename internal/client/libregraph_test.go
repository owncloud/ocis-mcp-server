package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testdataDir returns the absolute path to the testdata directory.
func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata")
}

func TestListJSON(t *testing.T) {
	type testUser struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
		Mail        string `json:"mail"`
	}

	tests := []struct {
		name      string
		fixture   string
		wantCount int
		wantFirst string
		wantErr   bool
	}{
		{
			name:      "parses users list from fixture",
			fixture:   "graph_users.json",
			wantCount: 3,
			wantFirst: "Albert Einstein",
		},
		{
			name:      "empty value array",
			fixture:   "", // inline empty
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.fixture != "" {
				var err error
				body, err = os.ReadFile(filepath.Join(testdataDir(), tt.fixture))
				if err != nil {
					t.Fatalf("reading fixture: %v", err)
				}
			} else {
				body = []byte(`{"value":[]}`)
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(body)
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			c := New(cfg)

			users, err := ListJSON[testUser](context.Background(), c, "/graph/v1.0/users", nil)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ListJSON error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if len(users) != tt.wantCount {
				t.Errorf("got %d users, want %d", len(users), tt.wantCount)
			}
			if tt.wantFirst != "" && len(users) > 0 {
				if users[0].DisplayName != tt.wantFirst {
					t.Errorf("first user = %q, want %q", users[0].DisplayName, tt.wantFirst)
				}
			}
		})
	}
}

func TestGetJSON(t *testing.T) {
	type testDrive struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		DriveType string `json:"driveType"`
	}

	tests := []struct {
		name     string
		response string
		wantID   string
		wantName string
		wantErr  bool
	}{
		{
			name:     "decodes single object",
			response: `{"id":"drive1","name":"Personal","driveType":"personal"}`,
			wantID:   "drive1",
			wantName: "Personal",
		},
		{
			name:     "decodes minimal object",
			response: `{"id":"drive2","name":"Shared","driveType":"virtual"}`,
			wantID:   "drive2",
			wantName: "Shared",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			c := New(cfg)

			drive, err := GetJSON[testDrive](context.Background(), c, "/graph/v1.0/drives/drive1", nil)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetJSON error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if drive.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", drive.ID, tt.wantID)
			}
			if drive.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", drive.Name, tt.wantName)
			}
		})
	}
}

func TestPostJSON(t *testing.T) {
	type createReq struct {
		DisplayName string `json:"displayName"`
		Mail        string `json:"mail"`
	}
	type createResp struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	}

	tests := []struct {
		name            string
		input           createReq
		wantContentType string
		wantID          string
		wantErr         bool
	}{
		{
			name:            "sends correct content-type and body",
			input:           createReq{DisplayName: "New User", Mail: "new@example.org"},
			wantContentType: "application/json",
			wantID:          "new-user-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotContentType = r.Header.Get("Content-Type")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":"new-user-id","displayName":"New User"}`))
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			c := New(cfg)

			resp, err := PostJSON[createResp](context.Background(), c, "/graph/v1.0/users", tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("PostJSON error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if gotContentType != tt.wantContentType {
				t.Errorf("Content-Type = %q, want %q", gotContentType, tt.wantContentType)
			}
			if resp.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", resp.ID, tt.wantID)
			}
		})
	}
}

func TestErrorResponses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantMsg    string
	}{
		{
			name:       "401 produces APIError",
			statusCode: 401,
			body:       `{}`,
			wantMsg:    "authentication failed",
		},
		{
			name:       "403 produces APIError",
			statusCode: 403,
			body:       `{"error":{"message":"access denied","code":"accessDenied"}}`,
			wantMsg:    "access denied",
		},
		{
			name:       "404 produces APIError",
			statusCode: 404,
			body:       `{}`,
			wantMsg:    "not found",
		},
		{
			name:       "500 produces APIError",
			statusCode: 500,
			body:       `{}`,
			wantMsg:    "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			c := New(cfg)

			type dummy struct{}
			_, err := GetJSON[dummy](context.Background(), c, "/graph/v1.0/test", nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			apiErr, ok := err.(*APIError)
			if !ok {
				t.Fatalf("expected *APIError, got %T: %v", err, err)
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

func TestDelete(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{name: "204 success", statusCode: 204, wantErr: false},
		{name: "200 success", statusCode: 200, wantErr: false},
		{name: "404 error", statusCode: 404, wantErr: true},
		{name: "500 error", statusCode: 500, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			c := New(newTestConfig(srv.URL))
			err := Delete(context.Background(), c, "/graph/v1.0/users/u1")
			if gotMethod != "DELETE" {
				t.Errorf("method = %q, want DELETE", gotMethod)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("Delete error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeleteWithHeaders(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header
		w.WriteHeader(204)
	}))
	defer srv.Close()

	c := New(newTestConfig(srv.URL))
	headers := map[string]string{
		"Purge": "T",
		"If":    `(<lock-token>)`,
	}
	err := DeleteWithHeaders(context.Background(), c, "/graph/v1.0/drives/d1", headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHeaders.Get("Purge") != "T" {
		t.Errorf("Purge header = %q, want T", gotHeaders.Get("Purge"))
	}
	if gotHeaders.Get("If") != `(<lock-token>)` {
		t.Errorf("If header = %q, want (<lock-token>)", gotHeaders.Get("If"))
	}
}

func TestDeleteWithHeadersError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()

	c := New(newTestConfig(srv.URL))
	err := DeleteWithHeaders(context.Background(), c, "/test", nil)
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestPatchJSON(t *testing.T) {
	type patchReq struct {
		DisplayName string `json:"displayName"`
	}
	type patchResp struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	}

	var gotMethod, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"u1","displayName":"Updated"}`))
	}))
	defer srv.Close()

	c := New(newTestConfig(srv.URL))
	resp, err := PatchJSON[patchResp](context.Background(), c, "/graph/v1.0/users/u1", patchReq{DisplayName: "Updated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "PATCH" {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if resp.DisplayName != "Updated" {
		t.Errorf("DisplayName = %q, want Updated", resp.DisplayName)
	}
}

func TestPatchJSONError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
	}))
	defer srv.Close()

	type dummy struct{}
	c := New(newTestConfig(srv.URL))
	_, err := PatchJSON[dummy](context.Background(), c, "/test", map[string]string{"a": "b"})
	if err == nil {
		t.Fatal("expected error for 409")
	}
}

func TestPostJSONRaw(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	defer srv.Close()

	c := New(newTestConfig(srv.URL))
	body, err := PostJSONRaw(context.Background(), c, "/api/test", map[string]string{"key": "val"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if !strings.Contains(string(body), "ok") {
		t.Errorf("body = %q, want to contain 'ok'", string(body))
	}
}

func TestPostJSONRawError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := New(newTestConfig(srv.URL))
	_, err := PostJSONRaw(context.Background(), c, "/test", nil)
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestGetRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("raw-response-bytes"))
	}))
	defer srv.Close()

	c := New(newTestConfig(srv.URL))
	body, status, err := GetRaw(context.Background(), c, "/ocs/v2.php/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
	if string(body) != "raw-response-bytes" {
		t.Errorf("body = %q, want raw-response-bytes", string(body))
	}
}

func TestGetRawReturnsStatusOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte("not found"))
	}))
	defer srv.Close()

	c := New(newTestConfig(srv.URL))
	body, status, err := GetRaw(context.Background(), c, "/missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 404 {
		t.Errorf("status = %d, want 404", status)
	}
	if string(body) != "not found" {
		t.Errorf("body = %q, want 'not found'", string(body))
	}
}

func TestGetJSONWithQuery(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	type dummy struct {
		ID string `json:"id"`
	}
	c := New(newTestConfig(srv.URL))
	q := url.Values{"$filter": []string{"displayName eq 'test'"}}
	_, err := GetJSON[dummy](context.Background(), c, "/graph/v1.0/users", q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotQuery, "filter=") {
		t.Errorf("query = %q, want to contain filter=", gotQuery)
	}
}

func TestListJSONWithQuery(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"value":[{"id":"1"},{"id":"2"}]}`))
	}))
	defer srv.Close()

	type dummy struct {
		ID string `json:"id"`
	}
	c := New(newTestConfig(srv.URL))
	q := url.Values{"$top": []string{"10"}}
	items, err := ListJSON[dummy](context.Background(), c, "/graph/v1.0/users", q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("got %d items, want 2", len(items))
	}
	if !strings.Contains(gotQuery, "top=10") {
		t.Errorf("query = %q, want to contain top=10", gotQuery)
	}
}
