package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
