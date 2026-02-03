package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type NotesTool struct {
	DataDir string
}

func NewNotesTool(dataDir string) *NotesTool {
	return &NotesTool{DataDir: dataDir}
}

func (t *NotesTool) Name() string {
	return "notes_append"
}

func (t *NotesTool) Description() string {
	return "Append a note to a specific section/file. Args: section, content."
}

type NotesInput struct {
	Content string `json:"content"`
}

func (t *NotesTool) Run(ctx context.Context, input json.RawMessage) (Result, error) {
	var in NotesInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Error: err.Error()}, err
	}
	if in.Content == "" {
		return Result{Error: "content is required"}, fmt.Errorf("content is required")
	}

	filename := filepath.Join(t.DataDir, "notes.txt")
	if err := os.MkdirAll(t.DataDir, 0755); err != nil {
		return Result{Error: err.Error()}, err
	}
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	defer f.Close()

	entry := fmt.Sprintf("[%s] %s\n", time.Now().Format(time.RFC3339), in.Content)
	if _, err := f.WriteString(entry); err != nil {
		return Result{Error: err.Error()}, err
	}

	return Result{Output: "Note appended successfully"}, nil
}

type NotesShowTool struct {
	DataDir string
}

func (t *NotesShowTool) Name() string { return "notes_show" }
func (t *NotesShowTool) Description() string {
	return "Show notes. Args: none."
}

func (t *NotesShowTool) Run(ctx context.Context, input json.RawMessage) (Result, error) {
	filename := filepath.Join(t.DataDir, "notes.txt")
	content, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{Output: "No notes found."}, nil
		}
		return Result{Error: err.Error()}, err
	}
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return Result{Output: "No notes found."}, nil
	}
	return Result{Output: "Notes:\n" + trimmed}, nil
}

type NotesClearTool struct {
	DataDir string
}

func (t *NotesClearTool) Name() string { return "notes_clear" }
func (t *NotesClearTool) Description() string {
	return "Clear all notes. Args: none."
}

func (t *NotesClearTool) Run(ctx context.Context, input json.RawMessage) (Result, error) {
	filename := filepath.Join(t.DataDir, "notes.txt")
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return Result{Error: err.Error()}, err
	}
	return Result{Output: "Notes cleared."}, nil
}
