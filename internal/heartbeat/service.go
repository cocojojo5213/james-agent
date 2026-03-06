package heartbeat

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cocojojo5213/james-agent/internal/shared"
)

type Service struct {
	workspace   string
	onHeartbeat func(prompt string) (string, error)
	interval    time.Duration
	skillsDir   string
}

func New(workspace string, onHB func(string) (string, error), interval time.Duration) *Service {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	return &Service{
		workspace:   workspace,
		onHeartbeat: onHB,
		interval:    interval,
		skillsDir:   filepath.Join(workspace, "skills"),
	}
}

func (s *Service) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	slog.Info("heartbeat started", "interval", s.interval)

	for {
		select {
		case <-ticker.C:
			s.tick()
		case <-ctx.Done():
			slog.Info("heartbeat stopped")
			return nil
		}
	}
}

func (s *Service) tick() {
	hbPath := filepath.Join(s.workspace, "HEARTBEAT.md")
	data, err := os.ReadFile(hbPath)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Error("heartbeat read error", "error", err)
		}
		return
	}

	heartbeatPrompt := strings.TrimSpace(string(data))
	if heartbeatPrompt == "" {
		return
	}

	// Build evolution context
	prompt := s.buildEvolutionContext(heartbeatPrompt)

	slog.Info("heartbeat triggering", "prompt_len", len(prompt))

	if s.onHeartbeat == nil {
		slog.Warn("heartbeat no handler set")
		return
	}

	result, err := s.onHeartbeat(prompt)
	if err != nil {
		slog.Error("heartbeat error", "error", err)
		return
	}

	if strings.Contains(result, "HEARTBEAT_OK") {
		slog.Debug("heartbeat: nothing to do")
	} else {
		slog.Info("heartbeat result", "result", shared.Truncate(result, 200))
	}
}

func (s *Service) buildEvolutionContext(heartbeatPrompt string) string {
	var sb strings.Builder
	sb.WriteString(heartbeatPrompt)
	sb.WriteString("\n\n## Current State\n")
	sb.WriteString(fmt.Sprintf("- Time: %s\n", time.Now().Format(time.RFC3339)))

	// Memory summary
	memPath := filepath.Join(s.workspace, "memory", "MEMORY.md")
	if memData, err := os.ReadFile(memPath); err == nil {
		memStr := strings.TrimSpace(string(memData))
		if memStr != "" {
			sb.WriteString(fmt.Sprintf("- Recent memory: %s\n", shared.Truncate(memStr, 300)))
		}
	}

	// Skills list
	if entries, err := os.ReadDir(s.skillsDir); err == nil {
		var skillNames []string
		for _, e := range entries {
			if e.IsDir() {
				skillNames = append(skillNames, e.Name())
			}
		}
		if len(skillNames) > 0 {
			sb.WriteString(fmt.Sprintf("- Skills loaded: %s\n", strings.Join(skillNames, ", ")))
		}
	}

	// Daily journal summary
	journalDir := filepath.Join(s.workspace, "journal")
	todayFile := filepath.Join(journalDir, time.Now().Format("2006-01-02")+".md")
	if jData, err := os.ReadFile(todayFile); err == nil {
		jStr := strings.TrimSpace(string(jData))
		if jStr != "" {
			sb.WriteString(fmt.Sprintf("- Today's conversations: %s\n", shared.Truncate(jStr, 300)))
		}
	}

	sb.WriteString("\n## You may modify these files to evolve:\n")
	sb.WriteString("- AGENTS.md, SOUL.md, skills/*/SKILL.md, memory/MEMORY.md\n")

	return sb.String()
}
