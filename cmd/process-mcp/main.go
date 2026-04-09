package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/hairglasses-studio/mcpkit/middleware/gate"
	"github.com/hairglasses-studio/mcpkit/observability"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/slogcfg"
	"github.com/hairglasses-studio/process-mcp/internal/process"
)

func main() {
	ctx := context.Background()

	// --- Logging ---
	slogcfg.Init(slogcfg.Config{
		ServiceName: "process-mcp",
	})

	// --- Observability ---
	obs, obsShutdown, err := observability.Init(ctx, observability.Config{
		ServiceName:    "process-mcp",
		ServiceVersion: "0.1.0",
		EnableMetrics:  true,
		EnableTracing:  true,
		OTLPEndpoint:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		PrometheusPort: "9097",
	})
	if err != nil {
		slog.Warn("failed to initialize observability", "error", err)
	}
	defer func() {
		if obsShutdown != nil {
			_ = obsShutdown(context.Background())
		}
	}()

	reg, s := process.Setup()

	// Apply observability middleware
	if obs != nil {
		reg.SetMiddleware([]registry.Middleware{
			obs.Middleware(),
			gate.Middleware(gate.Config{Gate: gate.PauseWrites}),
			registry.AuditMiddleware(""),
			registry.SafetyTierMiddleware(),
		})
	}

	if err := registry.ServeAuto(s); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
