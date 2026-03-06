package shared

import (
	"context"

	"github.com/cexll/agentsdk-go/pkg/api"
)

// Runtime interface for agent runtime (allows mocking in tests)
type Runtime interface {
	Run(ctx context.Context, req api.Request) (*api.Response, error)
	RunStream(ctx context.Context, req api.Request) (<-chan api.StreamEvent, error)
	Close()
}

// RuntimeAdapter wraps api.Runtime to implement Runtime interface
type RuntimeAdapter struct {
	RT *api.Runtime
}

func (r *RuntimeAdapter) Run(ctx context.Context, req api.Request) (*api.Response, error) {
	return r.RT.Run(ctx, req)
}

func (r *RuntimeAdapter) RunStream(ctx context.Context, req api.Request) (<-chan api.StreamEvent, error) {
	return r.RT.RunStream(ctx, req)
}

func (r *RuntimeAdapter) Close() {
	_ = r.RT.Close()
}
