package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/auth"
	"github.com/owncloud/ocis-mcp-server/internal/client"
	"github.com/owncloud/ocis-mcp-server/internal/config"
	"github.com/owncloud/ocis-mcp-server/internal/mcpserver"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}

	initLogger(cfg.LogLevel)

	if cfg.TLSSkipVerify {
		slog.Warn("TLS certificate verification is disabled (OCIS_MCP_TLS_SKIP_VERIFY=true)")
	}
	if cfg.Insecure {
		slog.Warn("insecure mode enabled — plaintext HTTP allowed (OCIS_MCP_INSECURE=true)")
	}

	for _, w := range cfg.Warnings {
		slog.Warn(w)
	}

	ocisClient := client.New(cfg)

	server := mcpserver.New(version, ocisClient, cfg)

	slog.Info("starting ocis-mcp-server",
		"version", version,
		"transport", cfg.Transport,
		"ocis_url", cfg.OcisBaseURL(),
		"auth_mode", cfg.AuthMode,
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch cfg.Transport {
	case "stdio":
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
			slog.Error("stdio server error", "error", err)
			os.Exit(1)
		}
	case "http":
		runHTTP(ctx, server, cfg)
	}
}

func runHTTP(ctx context.Context, server *mcp.Server, cfg *config.Config) {
	addr := cfg.HTTPAddr
	if !cfg.IsLoopbackBind() {
		slog.Warn("HTTP server bound to a non-loopback interface, ensure this is intentional and network-restricted",
			"addr", addr)
	}

	// Build the whole auth configuration before listening, so nothing is ever served
	// unauthenticated by accident.
	handler, err := buildHTTPHandler(ctx, server, cfg)
	if err != nil {
		slog.Error("HTTP transport configuration error", "mode", cfg.HTTPAuth, "error", err)
		os.Exit(1)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to listen", "addr", addr, "error", err)
		os.Exit(1)
	}

	authenticated := cfg.HTTPAuth == mcpserver.AuthModeOIDC || cfg.HTTPAuth == mcpserver.AuthModeSecret
	slog.Info("HTTP transport listening", "addr", addr, "auth_mode", cfg.HTTPAuth, "authenticated", authenticated)
	switch cfg.HTTPAuth {
	case mcpserver.AuthModeOIDC:
		slog.Info("OIDC inbound authentication enabled",
			"issuer", cfg.HTTPOIDCIssuer, "resource", cfg.ResourceURI())
	case mcpserver.AuthModeNone:
		slog.Warn("HTTP transport has no authentication (OCIS_MCP_HTTP_AUTH=none): reachable from this host only (loopback)",
			"addr", addr)
	}

	srv := mcpserver.NewHTTPServer(handler)
	go func() {
		<-ctx.Done()
		slog.Info("shutting down HTTP server")
		_ = srv.Close()
	}()

	if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
		slog.Error("HTTP server error", "error", err)
		os.Exit(1)
	}
}

// buildHTTPHandler builds the authenticated HTTP handler for the configured mode. In oidc
// mode it discovers the issuer (the discovery document and the jwks_uri) and fails if
// that is not possible; the signing keys themselves are fetched lazily on the first
// token and refreshed afterwards. Errors never contain a token or secret.
func buildHTTPHandler(ctx context.Context, server *mcp.Server, cfg *config.Config) (http.Handler, error) {
	a := mcpserver.HTTPAuth{Mode: cfg.HTTPAuth, Secret: cfg.HTTPSecret}
	if cfg.HTTPAuth == mcpserver.AuthModeOIDC {
		hc, err := auth.NewIssuerHTTPClient(cfg.HTTPOIDCCAFile)
		if err != nil {
			return nil, err
		}
		v, err := auth.NewVerifier(ctx, auth.VerifierOptions{
			Issuer:     cfg.HTTPOIDCIssuer,
			Resource:   cfg.ResourceURI(),
			HTTPClient: hc,
		})
		if err != nil {
			return nil, err
		}
		a.PublicURL = cfg.HTTPPublicURL
		a.Resource = cfg.ResourceURI()
		a.Issuer = cfg.HTTPOIDCIssuer
		a.MetadataURL = cfg.MetadataURL()
		a.Scopes = cfg.HTTPOIDCScopes
		a.Verifier = v.Verify
	}
	return mcpserver.NewHTTPHandler(server, a)
}

func initLogger(level string) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
	slog.SetDefault(logger)
}
