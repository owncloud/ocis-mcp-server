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
		slog.Warn("HTTP server bound to a non-loopback interface — ensure this is intentional and network-restricted",
			"addr", addr)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to listen", "addr", addr, "error", err)
		os.Exit(1)
	}
	slog.Info("HTTP transport listening", "addr", addr, "authenticated", cfg.HTTPAuthEnabled())
	if !cfg.HTTPAuthEnabled() {
		slog.Warn("HTTP transport is UNAUTHENTICATED (OCIS_MCP_HTTP_SECRET not set): any client that can reach this address can invoke every tool with the server's oCIS credentials. Set OCIS_MCP_HTTP_SECRET to require an 'Authorization: Bearer' token.",
			"addr", addr)
	}

	srv := mcpserver.NewHTTPServer(mcpserver.NewHTTPHandler(server, cfg.HTTPSecret))
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
