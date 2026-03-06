package shared

import (
	"context"
	"log/slog"
	"time"

	"github.com/cexll/agentsdk-go/pkg/api"
)

// LoggingRuntime wraps a Runtime and logs each call's duration and outcome.
type LoggingRuntime struct {
	Inner Runtime
}

func (l *LoggingRuntime) Run(ctx context.Context, req api.Request) (*api.Response, error) {
	start := time.Now()
	resp, err := l.Inner.Run(ctx, req)
	duration := time.Since(start)

	attrs := []any{
		"session", req.SessionID,
		"duration", duration.String(),
		"prompt_len", len(req.Prompt),
	}
	if err != nil {
		slog.Error("agent run failed", append(attrs, "error", err)...)
	} else {
		output := ""
		if resp != nil && resp.Result != nil {
			output = Truncate(resp.Result.Output, 100)
		}
		slog.Info("agent run completed", append(attrs, "output_preview", output)...)
	}
	return resp, err
}

func (l *LoggingRuntime) RunStream(ctx context.Context, req api.Request) (<-chan api.StreamEvent, error) {
	start := time.Now()
	slog.Info("agent stream started", "session", req.SessionID, "prompt_len", len(req.Prompt))

	ch, err := l.Inner.RunStream(ctx, req)
	if err != nil {
		slog.Error("agent stream failed",
			"session", req.SessionID,
			"duration", time.Since(start).String(),
			"error", err,
		)
		return nil, err
	}

	// Wrap channel to log completion
	out := make(chan api.StreamEvent, 16)
	go func() {
		defer close(out)
		for event := range ch {
			out <- event
		}
		slog.Info("agent stream completed",
			"session", req.SessionID,
			"duration", time.Since(start).String(),
		)
	}()

	return out, nil
}

func (l *LoggingRuntime) Close() {
	l.Inner.Close()
}
