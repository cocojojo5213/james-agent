package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cocojojo5213/james-agent/internal/shared"
)

type Journal struct {
	dir string
	mu  sync.Mutex
}

func New(workspace string) *Journal {
	dir := filepath.Join(workspace, "journal")
	return &Journal{dir: dir}
}

// Record appends a conversation summary entry to today's journal file.
func (j *Journal) Record(channel, senderID, prompt, result string) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if err := os.MkdirAll(j.dir, 0755); err != nil {
		return fmt.Errorf("create journal dir: %w", err)
	}

	filename := time.Now().Format("2006-01-02") + ".md"
	path := filepath.Join(j.dir, filename)

	entry := fmt.Sprintf(
		"### %s\n- Channel: %s\n- Sender: %s\n- Prompt: %s\n- Result: %s\n\n",
		time.Now().Format("15:04:05"),
		channel,
		senderID,
		shared.Truncate(prompt, 100),
		shared.Truncate(result, 200),
	)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open journal file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(entry)
	return err
}
