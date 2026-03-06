package shared

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cocojojo5213/james-agent/internal/memory"
)

// BuildSystemPrompt assembles the system prompt from AGENTS.md, SOUL.md, and memory context.
func BuildSystemPrompt(workspace string, mem *memory.MemoryStore) string {
	var sb strings.Builder

	if data, err := os.ReadFile(filepath.Join(workspace, "AGENTS.md")); err == nil {
		sb.Write(data)
		sb.WriteString("\n\n")
	}

	if data, err := os.ReadFile(filepath.Join(workspace, "SOUL.md")); err == nil {
		sb.Write(data)
		sb.WriteString("\n\n")
	}

	if memCtx := mem.GetMemoryContext(); memCtx != "" {
		sb.WriteString(memCtx)
	}

	return sb.String()
}
