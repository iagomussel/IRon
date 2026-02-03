package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestListTools(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	ctx := context.Background()

	addTool := &ListAddTool{BaseDir: baseDir}
	showTool := &ListShowTool{BaseDir: baseDir}
	removeTool := &ListRemoveTool{BaseDir: baseDir}
	listLists := &ListListsTool{BaseDir: baseDir}

	// No lists
	emptyLists, err := listLists.Run(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("list lists empty: %v", err)
	}
	if emptyLists.Output != "No lists found." {
		t.Fatalf("unexpected empty lists: %q", emptyLists.Output)
	}

	// Add
	addArgs, _ := json.Marshal(map[string]string{"list": "tasks", "item": "one"})
	if _, err := addTool.Run(ctx, addArgs); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Show
	showArgs, _ := json.Marshal(map[string]string{"list": "tasks"})
	showRes, err := showTool.Run(ctx, showArgs)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(showRes.Output, "tasks") || !strings.Contains(showRes.Output, "one") {
		t.Fatalf("unexpected show output: %q", showRes.Output)
	}

	// List lists
	listsRes, err := listLists.Run(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("list lists: %v", err)
	}
	if !strings.Contains(listsRes.Output, "tasks") {
		t.Fatalf("expected list name in output: %q", listsRes.Output)
	}

	// Remove
	removeArgs, _ := json.Marshal(map[string]string{"list": "tasks", "item": "one"})
	if _, err := removeTool.Run(ctx, removeArgs); err != nil {
		t.Fatalf("remove: %v", err)
	}

	// Show empty list
	showRes2, err := showTool.Run(ctx, showArgs)
	if err != nil {
		t.Fatalf("show after remove: %v", err)
	}
	if !strings.Contains(showRes2.Output, "empty") {
		t.Fatalf("expected empty output, got: %q", showRes2.Output)
	}
}

