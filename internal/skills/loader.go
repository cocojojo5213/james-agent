package skills

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cexll/agentsdk-go/pkg/api"
	runtimeskills "github.com/cexll/agentsdk-go/pkg/runtime/skills"
	"gopkg.in/yaml.v3"
)

const skillFileName = "SKILL.md"

var errInvalidSkillYAML = errors.New("invalid skill YAML frontmatter")

type skillFrontmatter struct {
	Name                   string   `yaml:"name"`
	Description            string   `yaml:"description"`
	Keywords               []string `yaml:"keywords"`
	CommandDispatch        string   `yaml:"command-dispatch"`
	CommandTool            string   `yaml:"command-tool"`
	DisableModelInvocation bool     `yaml:"disable-model-invocation"`
}

func LoadSkills(skillDir string) ([]api.SkillRegistration, error) {
	skillDir = strings.TrimSpace(skillDir)
	if skillDir == "" {
		return nil, nil
	}

	info, err := os.Stat(skillDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat skills dir %q: %w", skillDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skills path is not a directory: %s", skillDir)
	}

	entries, err := os.ReadDir(skillDir)
	if err != nil {
		return nil, fmt.Errorf("read skills dir %q: %w", skillDir, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	registrations := make([]api.SkillRegistration, 0, len(entries))
	seen := make(map[string]string, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(skillDir, entry.Name(), skillFileName)
		reg, skip, parseErr := parseSkillFile(skillPath)
		if parseErr != nil {
			return nil, parseErr
		}
		if skip {
			continue
		}

		if prevPath, exists := seen[reg.Definition.Name]; exists {
			return nil, fmt.Errorf("duplicate skill name %q in %s (already in %s)", reg.Definition.Name, skillPath, prevPath)
		}
		seen[reg.Definition.Name] = skillPath
		registrations = append(registrations, reg)
	}

	return registrations, nil
}

func parseSkillFile(path string) (api.SkillRegistration, bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return api.SkillRegistration{}, true, nil
		}
		return api.SkillRegistration{}, false, fmt.Errorf("read skill %q: %w", path, err)
	}

	meta, body, err := parseFrontmatter(content)
	if err != nil {
		if errors.Is(err, errInvalidSkillYAML) {
			slog.Warn("skip invalid YAML skill", "path", path, "error", err)
			return api.SkillRegistration{}, true, nil
		}
		return api.SkillRegistration{}, false, fmt.Errorf("parse skill %q: %w", path, err)
	}
	if strings.TrimSpace(meta.Name) == "" {
		return api.SkillRegistration{}, false, fmt.Errorf("parse skill %q: missing name", path)
	}

	body = strings.TrimSpace(body)
	def := runtimeskills.Definition{
		Name:        strings.TrimSpace(meta.Name),
		Description: strings.TrimSpace(meta.Description),
	}

	keywords := sanitizeKeywords(meta.Keywords)
	// Auto-extract keywords from description when none provided (OpenClaw compat)
	if len(keywords) == 0 && meta.Description != "" {
		keywords = extractKeywords(meta.Description)
	}
	if len(keywords) > 0 {
		def.Matchers = []runtimeskills.Matcher{
			runtimeskills.KeywordMatcher{Any: keywords},
		}
	}

	// When disable-model-invocation is true, skip injecting the body into the system prompt
	disableModel := meta.DisableModelInvocation
	handler := runtimeskills.HandlerFunc(func(context.Context, runtimeskills.ActivationContext) (runtimeskills.Result, error) {
		output := body
		if disableModel {
			output = ""
		}
		return runtimeskills.Result{
			Skill:  def.Name,
			Output: output,
			Metadata: map[string]any{
				"system_prompt":            body,
				"source_path":              path,
				"disable_model_invocation": disableModel,
				"command_dispatch":         meta.CommandDispatch,
				"command_tool":             meta.CommandTool,
			},
		}, nil
	})

	return api.SkillRegistration{Definition: def, Handler: handler}, false, nil
}

func parseFrontmatter(content []byte) (skillFrontmatter, string, error) {
	text := strings.TrimPrefix(string(content), "\uFEFF")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return skillFrontmatter{}, "", errors.New("missing YAML frontmatter")
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return skillFrontmatter{}, "", errors.New("missing closing frontmatter separator")
	}

	frontmatter := strings.Join(lines[1:end], "\n")
	body := strings.Join(lines[end+1:], "\n")

	var meta skillFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
		return skillFrontmatter{}, "", fmt.Errorf("%w: %v", errInvalidSkillYAML, err)
	}

	return meta, body, nil
}

func sanitizeKeywords(keywords []string) []string {
	if len(keywords) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(keywords))
	out := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		normalized := strings.ToLower(strings.TrimSpace(keyword))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)

	return out
}

// extractKeywords pulls significant words from a description string.
// Words shorter than 4 chars or common stop words are ignored.
func extractKeywords(desc string) []string {
	stopWords := map[string]struct{}{
		"this": {}, "that": {}, "with": {}, "from": {},
		"will": {}, "have": {}, "been": {}, "were": {},
		"they": {}, "them": {}, "their": {}, "what": {},
		"when": {}, "where": {}, "which": {}, "while": {},
		"does": {}, "done": {}, "about": {}, "also": {},
		"into": {}, "just": {}, "only": {}, "some": {},
		"than": {}, "then": {}, "very": {}, "such": {},
		"each": {}, "other": {}, "more": {}, "most": {},
	}

	words := strings.Fields(strings.ToLower(desc))
	seen := make(map[string]struct{}, len(words))
	var result []string
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?\"'()[]{}/-")
		if len(w) < 4 {
			continue
		}
		if _, stop := stopWords[w]; stop {
			continue
		}
		if _, dup := seen[w]; dup {
			continue
		}
		seen[w] = struct{}{}
		result = append(result, w)
		if len(result) >= 5 {
			break
		}
	}
	return result
}
