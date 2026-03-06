package heartbeat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cocojojo5213/james-agent/internal/shared"
)

func TestNew(t *testing.T) {
	s := New("/tmp/ws", nil, 0)
	if s == nil {
		t.Fatal("New returned nil")
	}
	if s.interval != 30*time.Minute {
		t.Errorf("default interval = %v, want 30m", s.interval)
	}
}

func TestNew_CustomInterval(t *testing.T) {
	s := New("/tmp/ws", nil, 5*time.Minute)
	if s.interval != 5*time.Minute {
		t.Errorf("interval = %v, want 5m", s.interval)
	}
}

func TestTick_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	var called atomic.Int32
	s := New(tmpDir, func(prompt string) (string, error) {
		called.Add(1)
		return "ok", nil
	}, time.Second)

	s.tick()

	if called.Load() != 0 {
		t.Error("handler should not be called when HEARTBEAT.md doesn't exist")
	}
}

func TestTick_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "HEARTBEAT.md"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	var called atomic.Int32
	s := New(tmpDir, func(prompt string) (string, error) {
		called.Add(1)
		return "ok", nil
	}, time.Second)

	s.tick()

	if called.Load() != 0 {
		t.Error("handler should not be called for empty HEARTBEAT.md")
	}
}

func TestTick_WithContent(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "HEARTBEAT.md"), []byte("Check tasks"), 0644)

	var receivedPrompt string
	s := New(tmpDir, func(prompt string) (string, error) {
		receivedPrompt = prompt
		return "done", nil
	}, time.Second)

	s.tick()

	if !strings.Contains(receivedPrompt, "Check tasks") {
		t.Errorf("prompt should contain 'Check tasks', got %q", receivedPrompt)
	}
	if !strings.Contains(receivedPrompt, "## Current State") {
		t.Errorf("prompt should contain evolution context, got %q", receivedPrompt)
	}
}

func TestStart_ContextCancel(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir, nil, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- s.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not exit after context cancel")
	}
}

func TestStart_TickerFires(t *testing.T) {
	tmpDir := t.TempDir()

	// Create HEARTBEAT.md
	_ = os.WriteFile(filepath.Join(tmpDir, "HEARTBEAT.md"), []byte("tick"), 0644)

	tickCount := 0
	s := New(tmpDir, func(prompt string) (string, error) {
		tickCount++
		return "ok", nil
	}, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- s.Start(ctx)
	}()

	// Wait for at least one tick
	time.Sleep(150 * time.Millisecond)
	cancel()

	<-done

	if tickCount == 0 {
		t.Error("expected at least one tick")
	}
}

func TestTick_HandlerError(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "HEARTBEAT.md"), []byte("Check tasks"), 0644)

	s := New(tmpDir, func(prompt string) (string, error) {
		return "", fmt.Errorf("handler error")
	}, time.Second)

	// Should not panic on handler error
	s.tick()
}

func TestTick_HeartbeatOK(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "HEARTBEAT.md"), []byte("Check tasks"), 0644)

	var called bool
	s := New(tmpDir, func(prompt string) (string, error) {
		called = true
		return "HEARTBEAT_OK - nothing to do", nil
	}, time.Second)

	s.tick()

	if !called {
		t.Error("handler should be called")
	}
}

func TestTick_NoHandler(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "HEARTBEAT.md"), []byte("Check tasks"), 0644)

	s := New(tmpDir, nil, time.Second)

	// Should not panic when handler is nil
	s.tick()
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is longer than ten", 10, "this is lo..."},
		{"", 5, ""},
	}

	for _, tt := range tests {
		got := shared.Truncate(tt.input, tt.n)
		if got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
		}
	}
}
