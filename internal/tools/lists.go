package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ListInput struct {
	List string `json:"list"`
	Item string `json:"item,omitempty"`
}

type ListAddTool struct {
	BaseDir string
}

func (t *ListAddTool) Name() string        { return "list_add" }
func (t *ListAddTool) Description() string { return "Add an item to a list. Args: list, item." }

func (t *ListAddTool) Run(ctx context.Context, input json.RawMessage) (Result, error) {
	var in ListInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Error: err.Error()}, err
	}
	if in.List == "" || in.Item == "" {
		return Result{Error: "list and item are required"}, fmt.Errorf("missing args")
	}

	path := t.getPath(in.List)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return Result{Error: err.Error()}, err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	defer f.Close()

	if _, err := f.WriteString(in.Item + "\n"); err != nil {
		return Result{Error: err.Error()}, err
	}

	return Result{Output: fmt.Sprintf("Added '%s' to list '%s'", in.Item, in.List)}, nil
}

func (t *ListAddTool) getPath(list string) string {
	return filepath.Join(t.BaseDir, "lists", list+".txt")
}

type ListRemoveTool struct {
	BaseDir string
}

func (t *ListRemoveTool) Name() string        { return "list_remove" }
func (t *ListRemoveTool) Description() string { return "Remove an item from a list. Args: list, item." }

func (t *ListRemoveTool) Run(ctx context.Context, input json.RawMessage) (Result, error) {
	var in ListInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Error: err.Error()}, err
	}
	if in.List == "" || in.Item == "" {
		return Result{Error: "list and item are required"}, fmt.Errorf("missing args")
	}

	path := filepath.Join(t.BaseDir, "lists", in.List+".txt")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{Error: "list not found"}, nil
		}
		return Result{Error: err.Error()}, err
	}

	lines := strings.Split(string(content), "\n")
	newLines := make([]string, 0, len(lines))
	removed := false
	for _, line := range lines {
		if line == "" {
			continue
		}
		if line == in.Item {
			removed = true
			continue
		}
		newLines = append(newLines, line)
	}

	if !removed {
		return Result{Output: fmt.Sprintf("Item '%s' not found in list '%s'", in.Item, in.List)}, nil
	}

	if len(newLines) > 0 {
		out := strings.Join(newLines, "\n") + "\n"
		if err := os.WriteFile(path, []byte(out), 0644); err != nil {
			return Result{Error: err.Error()}, err
		}
	} else {
		_ = os.Remove(path) // Remove empty list file? Or just clear it. Removing is cleaner.
	}

	return Result{Output: fmt.Sprintf("Removed '%s' from list '%s'", in.Item, in.List)}, nil
}

type ListShowTool struct {
	BaseDir string
}

func (t *ListShowTool) Name() string { return "list_show" }
func (t *ListShowTool) Description() string {
	return "Show items from a list. Args: list."
}

func (t *ListShowTool) Run(ctx context.Context, input json.RawMessage) (Result, error) {
	var in ListInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Error: err.Error()}, err
	}
	if in.List == "" {
		return Result{Error: "list is required"}, fmt.Errorf("missing args")
	}

	path := filepath.Join(t.BaseDir, "lists", in.List+".txt")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{Output: fmt.Sprintf("List '%s' is empty.", in.List)}, nil
		}
		return Result{Error: err.Error()}, err
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return Result{Output: fmt.Sprintf("List '%s' is empty.", in.List)}, nil
	}

	return Result{Output: fmt.Sprintf("List '%s':\n- %s", in.List, strings.Join(lines, "\n- "))}, nil
}
