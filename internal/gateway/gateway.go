package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cexll/agentsdk-go/pkg/api"
	"github.com/cexll/agentsdk-go/pkg/model"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/cocojojo5213/james-agent/internal/bus"
	"github.com/cocojojo5213/james-agent/internal/channel"
	"github.com/cocojojo5213/james-agent/internal/config"
	"github.com/cocojojo5213/james-agent/internal/cron"
	"github.com/cocojojo5213/james-agent/internal/heartbeat"
	"github.com/cocojojo5213/james-agent/internal/journal"
	"github.com/cocojojo5213/james-agent/internal/memory"
	modelprovider "github.com/cocojojo5213/james-agent/internal/provider"
	"github.com/cocojojo5213/james-agent/internal/shared"
	"github.com/cocojojo5213/james-agent/internal/skills"
)

// RuntimeFactory creates a Runtime instance
type RuntimeFactory func(cfg *config.Config, sysPrompt string) (shared.Runtime, error)

// Options for creating a Gateway
type Options struct {
	RuntimeFactory RuntimeFactory
	SignalChan     chan os.Signal // for testing signal handling
}

// DefaultRuntimeFactory creates the default agentsdk-go runtime
func DefaultRuntimeFactory(cfg *config.Config, sysPrompt string) (shared.Runtime, error) {
	return newRuntime(cfg, sysPrompt, nil)
}

func newRuntime(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration) (shared.Runtime, error) {
	providerFactory, err := modelprovider.BuildModelFactory(cfg)
	if err != nil {
		return nil, err
	}

	opts := api.Options{
		ProjectRoot:   cfg.Agent.Workspace,
		ModelFactory:  providerFactory,
		SystemPrompt:  sysPrompt,
		MaxIterations: cfg.Agent.MaxToolIterations,
		MCPServers:    cfg.MCP.Servers,
		TokenTracking: cfg.TokenTracking.Enabled,
		AutoCompact: api.CompactConfig{
			Enabled:       cfg.AutoCompact.Enabled,
			Threshold:     cfg.AutoCompact.Threshold,
			PreserveCount: cfg.AutoCompact.PreserveCount,
		},
		Skills: skillRegs,
	}

	rt, err := api.New(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("create runtime: %w", err)
	}
	return &shared.RuntimeAdapter{RT: rt}, nil
}

// PerSenderLimiter rate-limits messages per sender using a token bucket.
type PerSenderLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	r        rate.Limit
	burst    int
}

func NewPerSenderLimiter(r float64, burst int) *PerSenderLimiter {
	if r <= 0 {
		r = 0.5
	}
	if burst <= 0 {
		burst = 3
	}
	return &PerSenderLimiter{
		limiters: make(map[string]*rate.Limiter),
		r:        rate.Limit(r),
		burst:    burst,
	}
}

func (l *PerSenderLimiter) Allow(senderID string) bool {
	l.mu.Lock()
	limiter, ok := l.limiters[senderID]
	if !ok {
		limiter = rate.NewLimiter(l.r, l.burst)
		l.limiters[senderID] = limiter
	}
	l.mu.Unlock()
	return limiter.Allow()
}

type Gateway struct {
	cfg        *config.Config
	bus        *bus.MessageBus
	runtime    shared.Runtime
	channels   *channel.ChannelManager
	cron       *cron.Service
	hb         *heartbeat.Service
	mem        *memory.MemoryStore
	skillRegs  []api.SkillRegistration
	signalChan chan os.Signal // for testing
	startTime  time.Time
	limiter    *PerSenderLimiter
	httpServer *http.Server
	journal    *journal.Journal
}

// New creates a Gateway with default options
func New(cfg *config.Config) (*Gateway, error) {
	return NewWithOptions(cfg, Options{})
}

// NewWithOptions creates a Gateway with custom options for testing
func NewWithOptions(cfg *config.Config, opts Options) (*Gateway, error) {
	g := &Gateway{cfg: cfg}

	// Message bus
	g.bus = bus.NewMessageBus(config.DefaultBufSize)

	// Memory
	g.mem = memory.NewMemoryStore(cfg.Agent.Workspace)

	// Conversation journal
	g.journal = journal.New(cfg.Agent.Workspace)

	// Build system prompt
	sysPrompt := shared.BuildSystemPrompt(cfg.Agent.Workspace, g.mem)

	if cfg.Skills.Enabled {
		skillDir := cfg.Skills.Dir
		if skillDir == "" {
			skillDir = filepath.Join(cfg.Agent.Workspace, "skills")
		}
		skillRegs, err := skills.LoadSkills(skillDir)
		if err != nil {
			slog.Warn("skills load warning", "error", err)
		}
		g.skillRegs = skillRegs
	}

	// Rate limiter
	g.limiter = NewPerSenderLimiter(cfg.Gateway.RateLimit, cfg.Gateway.RateBurst)
	g.startTime = time.Now()

	// Create runtime using factory (allows injection for testing)
	factory := opts.RuntimeFactory
	var (
		rt  shared.Runtime
		err error
	)
	if factory == nil {
		rt, err = newRuntime(cfg, sysPrompt, g.skillRegs)
	} else {
		rt, err = factory(cfg, sysPrompt)
	}
	if err != nil {
		return nil, err
	}
	// Wrap runtime with logging middleware
	g.runtime = &shared.LoggingRuntime{Inner: rt}

	// Signal channel for testing
	g.signalChan = opts.SignalChan

	// runAgent helper for cron/heartbeat
	runAgent := func(prompt string) (string, error) {
		return g.runAgent(context.Background(), prompt, "system", nil)
	}

	// Cron
	cronStorePath := filepath.Join(config.ConfigDir(), "data", "cron", "jobs.json")
	g.cron = cron.NewService(cronStorePath)
	g.cron.OnJob = func(job cron.CronJob) (string, error) {
		result, err := runAgent(job.Payload.Message)
		if err != nil {
			return "", err
		}
		if job.Payload.Deliver && job.Payload.Channel != "" {
			g.bus.Outbound <- bus.OutboundMessage{
				Channel: job.Payload.Channel,
				ChatID:  job.Payload.To,
				Content: result,
			}
		}
		return result, nil
	}

	// Heartbeat
	g.hb = heartbeat.New(cfg.Agent.Workspace, runAgent, 0)

	// Channels (with gateway config for WebUI port)
	chMgr, err := channel.NewChannelManagerWithGateway(cfg.Channels, cfg.Gateway, g.bus)
	if err != nil {
		return nil, fmt.Errorf("create channel manager: %w", err)
	}
	g.channels = chMgr

	return g, nil
}

func (g *Gateway) mergeContentBlocks(prompt string, contentBlocks []model.ContentBlock) (string, []model.ContentBlock) {
	if len(contentBlocks) > 0 && strings.TrimSpace(prompt) != "" {
		blocks := make([]model.ContentBlock, 0, len(contentBlocks)+1)
		blocks = append(blocks, model.ContentBlock{Type: model.ContentBlockText, Text: prompt})
		blocks = append(blocks, contentBlocks...)
		return "", blocks
	}
	return prompt, contentBlocks
}

func (g *Gateway) runAgent(ctx context.Context, prompt, sessionID string, contentBlocks []model.ContentBlock) (string, error) {
	prompt, blocks := g.mergeContentBlocks(prompt, contentBlocks)

	resp, err := g.runtime.Run(ctx, api.Request{
		Prompt:        prompt,
		ContentBlocks: blocks,
		SessionID:     sessionID,
	})
	if err != nil {
		return "", err
	}
	if resp == nil || resp.Result == nil {
		return "", nil
	}
	return resp.Result.Output, nil
}

// runAgentStream executes the agent with streaming, calling onDelta for each text chunk
// and onToolStart when a tool begins execution.
func (g *Gateway) runAgentStream(ctx context.Context, prompt, sessionID string,
	contentBlocks []model.ContentBlock,
	onDelta func(text string),
	onToolStart func(toolName string),
) (string, error) {
	prompt, blocks := g.mergeContentBlocks(prompt, contentBlocks)

	events, err := g.runtime.RunStream(ctx, api.Request{
		Prompt:        prompt,
		ContentBlocks: blocks,
		SessionID:     sessionID,
	})
	if err != nil {
		return "", err
	}

	var full strings.Builder
	for event := range events {
		switch event.Type {
		case api.EventContentBlockDelta:
			if event.Delta != nil {
				full.WriteString(event.Delta.Text)
				if onDelta != nil {
					onDelta(event.Delta.Text)
				}
			}
		case api.EventToolExecutionStart:
			if onToolStart != nil {
				onToolStart(event.Name)
			}
		}
	}
	return full.String(), nil
}

func (g *Gateway) Run(ctx context.Context) error {
	// Use injected signal channel for testing, or create default
	var cancel context.CancelFunc
	if g.signalChan != nil {
		ctx, cancel = context.WithCancel(ctx)
		go func() {
			<-g.signalChan
			cancel()
		}()
	} else {
		ctx, cancel = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	}
	defer cancel()

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error { g.bus.DispatchOutbound(ctx); return nil })

	eg.Go(func() error {
		if err := g.channels.StartAll(ctx); err != nil {
			return fmt.Errorf("start channels: %w", err)
		}
		slog.Info("channels started", "channels", g.channels.EnabledChannels())
		return nil
	})

	eg.Go(func() error {
		if err := g.cron.Start(ctx); err != nil {
			slog.Warn("cron start warning", "error", err)
		}
		return nil
	})

	eg.Go(func() error {
		if err := g.hb.Start(ctx); err != nil {
			slog.Error("heartbeat error", "error", err)
		}
		return nil
	})

	eg.Go(func() error { return g.processLoop(ctx) })

	// Start health check HTTP server
	eg.Go(func() error { return g.startHTTPServer(ctx) })

	slog.Info("gateway running", "host", g.cfg.Gateway.Host, "port", g.cfg.Gateway.Port)

	waitErr := eg.Wait()
	if waitErr != nil {
		slog.Error("gateway error", "error", waitErr)
	}

	slog.Info("shutting down")
	shutdownErr := g.Shutdown()
	if waitErr != nil {
		return waitErr
	}
	return shutdownErr
}

func (g *Gateway) startHTTPServer(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", g.healthHandler)
	mux.HandleFunc("POST /v1/chat/completions", g.chatCompletionsHandler)

	g.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", g.cfg.Gateway.Host, g.cfg.Gateway.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = g.httpServer.Shutdown(shutdownCtx)
	}()

	slog.Info("health server listening", "addr", g.httpServer.Addr)
	if err := g.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

func (g *Gateway) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":   "ok",
		"uptime":   time.Since(g.startTime).String(),
		"channels": g.channels.EnabledChannels(),
	})
}

type chatCompletionRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Stream bool `json:"stream"`
}

func (g *Gateway) chatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	var req chatCompletionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, `{"error":{"message":"invalid request body"}}`, http.StatusBadRequest)
		return
	}

	var prompt strings.Builder
	for _, m := range req.Messages {
		if m.Role == "user" || m.Role == "system" {
			prompt.WriteString(m.Content)
			prompt.WriteString("\n")
		}
	}

	if req.Stream {
		g.handleStreamCompletion(w, r, prompt.String())
		return
	}

	result, err := g.runAgent(r.Context(), prompt.String(), "api-"+r.RemoteAddr, nil)
	if err != nil {
		slog.Error("chat completion error", "error", err)
		http.Error(w, `{"error":{"message":"internal error"}}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   g.cfg.Agent.Model,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": result,
				},
				"finish_reason": "stop",
			},
		},
	})
}

func (g *Gateway) handleStreamCompletion(w http.ResponseWriter, r *http.Request, prompt string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":{"message":"streaming not supported"}}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	completionID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())

	_, err := g.runAgentStream(r.Context(), prompt, "api-"+r.RemoteAddr, nil,
		func(delta string) {
			chunk := map[string]any{
				"id":      completionID,
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   g.cfg.Agent.Model,
				"choices": []map[string]any{
					{
						"index": 0,
						"delta": map[string]string{
							"content": delta,
						},
					},
				},
			}
			data, _ := json.Marshal(chunk)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		},
		nil,
	)

	if err != nil {
		slog.Error("stream completion error", "error", err)
	}

	_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (g *Gateway) processLoop(ctx context.Context) error {
	maxConcurrent := g.cfg.Gateway.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	sem := make(chan struct{}, maxConcurrent)

	for {
		select {
		case msg := <-g.bus.Inbound:
			slog.Info("inbound message",
				"channel", msg.Channel,
				"sender", msg.SenderID,
				"preview", shared.Truncate(msg.Content, 80),
			)

			if !g.limiter.Allow(msg.SenderID) {
				slog.Warn("rate limited", "sender", msg.SenderID)
				g.bus.Outbound <- bus.OutboundMessage{
					Channel: msg.Channel,
					ChatID:  msg.ChatID,
					Content: "You're sending messages too fast. Please wait a moment.",
				}
				continue
			}

			sem <- struct{}{}
			go func(m bus.InboundMessage) {
				defer func() { <-sem }()
				g.handleMessage(ctx, m)
			}(msg)
		case <-ctx.Done():
			return nil
		}
	}
}

func (g *Gateway) handleMessage(ctx context.Context, msg bus.InboundMessage) {
	if msg.Channel == "telegram" {
		g.handleTelegramStream(ctx, msg)
		return
	}

	result, err := g.runAgent(ctx, msg.Content, msg.SessionKey(), msg.ContentBlocks)
	if err != nil {
		slog.Error("agent error", "error", err, "channel", msg.Channel)
		result = "Sorry, I encountered an error processing your message."
	}
	if result != "" {
		g.bus.Outbound <- bus.OutboundMessage{
			Channel: msg.Channel,
			ChatID:  msg.ChatID,
			Content: result,
		}
	}

	if err := g.journal.Record(msg.Channel, msg.SenderID, msg.Content, result); err != nil {
		slog.Warn("journal record failed", "error", err)
	}
}

// handleTelegramStream processes a Telegram message with streaming via RunStream.
// It sends a placeholder message, then progressively updates it as content arrives.
func (g *Gateway) handleTelegramStream(ctx context.Context, msg bus.InboundMessage) {
	result, err := g.runAgentStream(ctx, msg.Content, msg.SessionKey(), msg.ContentBlocks,
		func(delta string) {
			// Streaming deltas are handled by the Telegram channel's progressive sender.
			// For now, we collect and send via the outbound bus after completion.
		},
		func(toolName string) {
			slog.Debug("tool execution", "tool", toolName, "session", msg.SessionKey())
		},
	)

	if err != nil {
		slog.Error("agent stream error", "error", err, "channel", msg.Channel)
		result = "Sorry, I encountered an error processing your message."
	}

	if result != "" {
		g.bus.Outbound <- bus.OutboundMessage{
			Channel: msg.Channel,
			ChatID:  msg.ChatID,
			Content: result,
		}
	}

	if err := g.journal.Record(msg.Channel, msg.SenderID, msg.Content, result); err != nil {
		slog.Warn("journal record failed", "error", err)
	}
}

func (g *Gateway) Shutdown() error {
	g.cron.Stop()
	_ = g.channels.StopAll()
	if g.httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = g.httpServer.Shutdown(shutdownCtx)
	}
	if g.runtime != nil {
		g.runtime.Close()
	}
	slog.Info("shutdown complete")
	return nil
}
