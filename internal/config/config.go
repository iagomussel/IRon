package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type TaskConfig struct {
	ID         string   `json:"id"`
	Cron       string   `json:"cron"`
	Prompt     string   `json:"prompt"`
	SessionKey string   `json:"session_key"`
	Adapter    string   `json:"adapter"`
	Targets    []string `json:"targets"`
}

type AddonConfig struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"` // tool | adapter
	Repo      string   `json:"repo"`
	Build     []string `json:"build"`
	Binary    string   `json:"binary"`
	ToolName  string   `json:"tool_name"`
	AdapterID string   `json:"adapter_id"`
}

type Config struct {
	TelegramToken   string        `json:"telegram_token"`
	AllowedChatIDs  []int64       `json:"allowed_chat_ids"`
	CodexCommand    []string      `json:"codex_command"`
	CodexEnv        []string      `json:"codex_env"`
	DataDir         string        `json:"data_dir"`
	ToolsAddr       string        `json:"tools_addr"`
	Tasks           []TaskConfig  `json:"tasks"`
	Addons          []AddonConfig `json:"addons"`
	MaxResponseSize int           `json:"max_response_size"`
}

func DefaultConfig() Config {
	return Config{
		TelegramToken:   os.Getenv("TELEGRAM_TOKEN"),
		AllowedChatIDs:  parseChatIDs(os.Getenv("TELEGRAM_ALLOWED_CHAT_IDS")),
		CodexCommand:    defaultCodexCommand(),
		CodexEnv:        parseEnvList(os.Getenv("CODEX_ENV")),
		DataDir:         "data",
		ToolsAddr:       ":8089",
		MaxResponseSize: 3500,
	}
}

func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	applyEnvOverrides(&cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("TELEGRAM_TOKEN"); v != "" {
		cfg.TelegramToken = v
	}
	if v := os.Getenv("TELEGRAM_ALLOWED_CHAT_IDS"); v != "" {
		cfg.AllowedChatIDs = parseChatIDs(v)
	}
	if v := os.Getenv("CODEX_COMMAND"); v != "" {
		cfg.CodexCommand = strings.Fields(v)
	}
	if v := os.Getenv("CODEX_ENV"); v != "" {
		cfg.CodexEnv = parseEnvList(v)
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("TOOLS_ADDR"); v != "" {
		cfg.ToolsAddr = v
	}
	if v := os.Getenv("MAX_RESPONSE_SIZE"); v != "" {
		if n, err := parseInt(v); err == nil {
			cfg.MaxResponseSize = n
		}
	}
}

func defaultCodexCommand() []string {
	if v := os.Getenv("CODEX_COMMAND"); v != "" {
		return strings.Fields(v)
	}
	return []string{"codex", "exec", "--dangerously-bypass-approvals-and-sandbox", "-"}
}

func parseEnvList(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseChatIDs(raw string) []int64 {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if n, err := parseInt64(p); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
