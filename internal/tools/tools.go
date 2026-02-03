package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agentic/internal/executil"
)

type Tool interface {
	Name() string
	Run(ctx context.Context, input json.RawMessage) (Result, error)
}

type Result struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) Tool {
	return r.tools[name]
}

func (r *Registry) List() []string {
	out := make([]string, 0, len(r.tools))
	for name := range r.tools {
		out = append(out, name)
	}
	return out
}

type HTTPFetchInput struct {
	URL        string `json:"url"`
	UserAgent  string `json:"user_agent"`
	MaxBytes   int64  `json:"max_bytes"`
	TimeoutSec int    `json:"timeout_sec"`
}

type HTTPFetchTool struct{}

func (t *HTTPFetchTool) Name() string { return "http_fetch" }

func (t *HTTPFetchTool) Run(ctx context.Context, input json.RawMessage) (Result, error) {
	var in HTTPFetchInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Error: err.Error()}, err
	}
	if in.URL == "" {
		return Result{Error: "url is required"}, errors.New("url is required")
	}
	if in.MaxBytes <= 0 {
		in.MaxBytes = 200000
	}
	if in.TimeoutSec <= 0 {
		in.TimeoutSec = 20
	}
	client := &http.Client{Timeout: time.Duration(in.TimeoutSec) * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	if in.UserAgent != "" {
		req.Header.Set("User-Agent", in.UserAgent)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	defer resp.Body.Close()
	reader := io.LimitReader(resp.Body, in.MaxBytes)
	body, err := io.ReadAll(reader)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	return Result{Output: string(body)}, nil
}

type ShellExecInput struct {
	Command    []string `json:"command"`
	TimeoutSec int      `json:"timeout_sec"`
}

type ShellExecTool struct{}

func (t *ShellExecTool) Name() string { return "shell_exec" }

func (t *ShellExecTool) Run(ctx context.Context, input json.RawMessage) (Result, error) {
	var in ShellExecInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Error: err.Error()}, err
	}
	if len(in.Command) == 0 {
		return Result{Error: "command is required"}, errors.New("command is required")
	}
	if in.TimeoutSec <= 0 {
		in.TimeoutSec = 60
	}
	res, err := executil.Run(ctx, in.Command[0], in.Command[1:], nil, nil, time.Duration(in.TimeoutSec)*time.Second, "")
	out := strings.TrimSpace(res.Stdout)
	if res.Stderr != "" {
		out = strings.TrimSpace(out + "\n" + res.Stderr)
	}
	if err != nil {
		return Result{Output: out, Error: err.Error()}, err
	}
	return Result{Output: out}, nil
}

type DockerExecInput struct {
	Args []string `json:"args"`
}

type DockerExecTool struct{}

func (t *DockerExecTool) Name() string { return "docker_exec" }

func (t *DockerExecTool) Run(ctx context.Context, input json.RawMessage) (Result, error) {
	var in DockerExecInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Error: err.Error()}, err
	}
	if len(in.Args) == 0 {
		return Result{Error: "args is required"}, errors.New("args is required")
	}
	res, err := executil.Run(ctx, "docker", in.Args, nil, nil, 5*time.Minute, "")
	out := strings.TrimSpace(res.Stdout)
	if res.Stderr != "" {
		out = strings.TrimSpace(out + "\n" + res.Stderr)
	}
	if err != nil {
		return Result{Output: out, Error: err.Error()}, err
	}
	return Result{Output: out}, nil
}

type CodeExecInput struct {
	Language   string   `json:"language"`
	Code       string   `json:"code"`
	Args       []string `json:"args"`
	TimeoutSec int      `json:"timeout_sec"`
}

type CodeExecTool struct{}

func (t *CodeExecTool) Name() string { return "code_exec" }

func (t *CodeExecTool) Run(ctx context.Context, input json.RawMessage) (Result, error) {
	var in CodeExecInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Error: err.Error()}, err
	}
	if in.Code == "" || in.Language == "" {
		return Result{Error: "language and code are required"}, errors.New("language and code are required")
	}
	if in.TimeoutSec <= 0 {
		in.TimeoutSec = 60
	}
	workDir, err := os.MkdirTemp("", "agentic-code-*")
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	defer os.RemoveAll(workDir)

	var cmd string
	var args []string
	var filename string
	var content string
	var runArgs []string

	switch strings.ToLower(in.Language) {
	case "python", "py":
		filename = "main.py"
		cmd = "python3"
		args = []string{filename}
		content = in.Code
	case "bash", "sh":
		filename = "script.sh"
		cmd = "bash"
		args = []string{filename}
		content = in.Code
	case "go", "golang":
		filename = "main.go"
		cmd = "go"
		args = []string{"run", filename}
		content = in.Code
	default:
		return Result{Error: "unsupported language"}, errors.New("unsupported language")
	}

	if err := os.WriteFile(filepath.Join(workDir, filename), []byte(content), 0o644); err != nil {
		return Result{Error: err.Error()}, err
	}
	if len(in.Args) > 0 {
		runArgs = append(args, in.Args...)
	} else {
		runArgs = args
	}
	cmdExec := exec.CommandContext(ctx, cmd, runArgs...)
	cmdExec.Dir = workDir
	var stdout strings.Builder
	var stderr strings.Builder
	cmdExec.Stdout = &stdout
	cmdExec.Stderr = &stderr
	if err := cmdExec.Start(); err != nil {
		return Result{Error: err.Error()}, err
	}
	done := make(chan error, 1)
	go func() {
		done <- cmdExec.Wait()
	}()
	select {
	case err := <-done:
		out := strings.TrimSpace(stdout.String())
		if stderr.Len() > 0 {
			out = strings.TrimSpace(out + "\n" + stderr.String())
		}
		if err != nil {
			return Result{Output: out, Error: err.Error()}, err
		}
		return Result{Output: out}, nil
	case <-time.After(time.Duration(in.TimeoutSec) * time.Second):
		_ = cmdExec.Process.Kill()
		return Result{Error: "execution timed out"}, errors.New("execution timed out")
	}
}

// ExternalTool wraps an executable that reads JSON input and writes JSON output.
type ExternalTool struct {
	ToolName string
	Command  []string
	Timeout  time.Duration
}

func (t *ExternalTool) Name() string { return t.ToolName }

func (t *ExternalTool) Run(ctx context.Context, input json.RawMessage) (Result, error) {
	if len(t.Command) == 0 {
		return Result{Error: "command is required"}, errors.New("command is required")
	}
	res, err := executil.Run(ctx, t.Command[0], t.Command[1:], input, nil, t.Timeout, "")
	if err != nil {
		return Result{Output: strings.TrimSpace(res.Stdout), Error: err.Error()}, err
	}
	var out Result
	if err := json.Unmarshal([]byte(res.Stdout), &out); err == nil && out.Output != "" {
		return out, nil
	}
	return Result{Output: strings.TrimSpace(res.Stdout)}, nil
}

func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(&HTTPFetchTool{})
	r.Register(&ShellExecTool{})
	r.Register(&DockerExecTool{})
	r.Register(&CodeExecTool{})
	return r
}

func FormatToolList(list []string) string {
	if len(list) == 0 {
		return "no tools registered"
	}
	return fmt.Sprintf("tools: %s", strings.Join(list, ", "))
}
