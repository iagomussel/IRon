package addons

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"agentic/internal/adapters"
	"agentic/internal/config"
	"agentic/internal/tools"
)

type Manager struct {
	RootDir string
}

func New(root string) *Manager {
	return &Manager{RootDir: root}
}

func (m *Manager) Load(ctx context.Context, addons []config.AddonConfig, toolReg *tools.Registry, adapterReg *adapters.Registry) error {
	for _, addon := range addons {
		if addon.Name == "" || addon.Repo == "" {
			continue
		}
		localDir := filepath.Join(m.RootDir, addon.Name)
		if _, err := os.Stat(localDir); os.IsNotExist(err) {
			cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", addon.Repo, localDir)
			if err := cmd.Run(); err != nil {
				return err
			}
		}
		if len(addon.Build) > 0 {
			cmd := exec.CommandContext(ctx, addon.Build[0], addon.Build[1:]...)
			cmd.Dir = localDir
			if err := cmd.Run(); err != nil {
				return err
			}
		}
		if addon.Binary == "" {
			return errors.New("addon binary is required")
		}
		bin := addon.Binary
		if !filepath.IsAbs(bin) {
			bin = filepath.Join(localDir, addon.Binary)
		}
		switch addon.Type {
		case "tool":
			name := addon.ToolName
			if name == "" {
				name = addon.Name
			}
			toolReg.Register(&tools.ExternalTool{ToolName: name, Command: []string{bin}, Timeout: 2 * time.Minute})
		case "adapter":
			id := addon.AdapterID
			if id == "" {
				id = addon.Name
			}
			adapterReg.Register(&adapters.ExternalAdapter{AdapterID: id, Command: []string{bin}, Timeout: 2 * time.Minute})
		}
	}
	return nil
}
