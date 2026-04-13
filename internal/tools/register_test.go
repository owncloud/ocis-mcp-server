package tools

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

func TestRegisterAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := newTestConfig(srv.URL)
	c := client.New(cfg)
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)

	// Should not panic.
	RegisterAll(s, c, cfg)
}

func TestRegisterAllWithEducation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := newTestConfig(srv.URL)
	cfg.EducationAccessToken = "edu-token"
	c := client.New(cfg)
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)

	// Should not panic and should register education tools.
	RegisterAll(s, c, cfg)
}

func TestRegisterEducationDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)

	// Should not panic and should skip education tools.
	registerEducation(s, c, false)
}

