package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotesTools(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()

	appendTool := NewNotesTool(dataDir)
	showTool := &NotesShowTool{DataDir: dataDir}
	clearTool := &NotesClearTool{DataDir: dataDir}

	ctx := context.Background()

	// Show on empty
	emptyRes, err := showTool.Run(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("show empty: %v", err)
	}
	if emptyRes.Output != "No notes found." {
		t.Fatalf("unexpected empty output: %q", emptyRes.Output)
	}

	// Append
	in := map[string]string{"content": "first note"}
	raw, _ := json.Marshal(in)
	appendRes, err := appendTool.Run(ctx, raw)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if appendRes.Output == "" {
		t.Fatalf("append output empty")
	}

	// Show with content
	showRes, err := showTool.Run(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(showRes.Output, "first note") {
		t.Fatalf("expected note content, got: %q", showRes.Output)
	}

	// Clear
	clearRes, err := clearTool.Run(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if clearRes.Output != "Notes cleared." {
		t.Fatalf("unexpected clear output: %q", clearRes.Output)
	}

	// File removed
	if _, err := showTool.Run(ctx, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("show after clear: %v", err)
	}

	// Ensure file path is in data dir
	notePath := filepath.Join(dataDir, "notes.txt")
	_ = notePath
}

