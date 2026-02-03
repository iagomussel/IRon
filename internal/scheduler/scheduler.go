package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agentic/internal/adapters"
	"agentic/internal/codex"
	"agentic/internal/config"

	"agentic/internal/tools"

	"github.com/robfig/cron/v3"
)

type JobStore interface {
	Add(task config.TaskConfig) error
	List() ([]config.TaskConfig, error)
}

type FileJobStore struct {
	path string
	mu   sync.Mutex
}

func NewFileJobStore(dir string) *FileJobStore {
	return &FileJobStore{path: filepath.Join(dir, "jobs.json")}
}

func (s *FileJobStore) Add(task config.TaskConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tasks, _ := s.List()
	tasks = append(tasks, task)
	return s.save(tasks)
}

func (s *FileJobStore) List() ([]config.TaskConfig, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []config.TaskConfig{}, nil
		}
		return nil, err
	}
	var tasks []config.TaskConfig
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *FileJobStore) save(tasks []config.TaskConfig) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

type Scheduler struct {
	cron     *cron.Cron
	codex    *codex.Client
	adapters *adapters.Registry
	tools    *tools.Registry
	store    JobStore
}

func New(codexClient *codex.Client, adaptersReg *adapters.Registry, toolsReg *tools.Registry, dataDir string) *Scheduler {
	// Standard parser (Minute Hour Dom Month Dow)
	s := &Scheduler{
		cron:     cron.New(),
		codex:    codexClient,
		adapters: adaptersReg,
		tools:    toolsReg,
		store:    NewFileJobStore(dataDir),
	}

	// Load persisted tasks
	if tasks, err := s.store.List(); err == nil {
		_ = s.RegisterTasks(tasks)
	}

	return s
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Stop(ctx context.Context) error {
	return s.cron.Stop().Err()
}

func (s *Scheduler) RegisterTasks(tasks []config.TaskConfig) error {
	for _, task := range tasks {
		task := task
		_, err := s.cron.AddFunc(task.Cron, func() {
			if err := s.runTask(task); err != nil {
				log.Printf("task %s failed: %v", task.ID, err)
			}
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) runTask(task config.TaskConfig) error {
	var toolOutputs strings.Builder
	hasTools := len(task.Tools) > 0
	hasPrompt := task.Prompt != ""

	// 1. Execute Tools (if any)
	if hasTools {
		for _, req := range task.Tools {
			tool := s.tools.Get(req.Name)
			if tool == nil {
				log.Printf("task %s: tool not found: %s", task.ID, req.Name)
				toolOutputs.WriteString(fmt.Sprintf("[Error] Tool %s not found\n", req.Name))
				continue
			}
			res, err := tool.Run(context.Background(), req.Args)
			output := res.Output
			if err != nil {
				output = fmt.Sprintf("Error: %v", err)
			}

			// Capture output
			toolOutputs.WriteString(fmt.Sprintf("Tool '%s' Output:\n%s\n\n", req.Name, output))

			// Mode 1: Tools ONLY (No Prompt) -> Send outputs immediately as they come (or batched? immediate is fine)
			if !hasPrompt {
				if adapter := s.adapters.Get(task.Adapter); adapter != nil {
					for _, target := range task.Targets {
						_ = adapter.Send(context.Background(), target, fmt.Sprintf("[%s] %s", req.Name, output))
					}
				}
			}
		}
	}

	// Mode 2 & 3: LLM (with or without tool context)
	if hasPrompt {
		fullPrompt := task.Prompt
		if toolOutputs.Len() > 0 {
			fullPrompt += "\n\n=== Context from scheduled tools ===\n" + toolOutputs.String()
		}

		resp, err := s.codex.Exec(context.Background(), "", "", fullPrompt, true)
		if err != nil {
			return err
		}

		adapter := s.adapters.Get(task.Adapter)
		if adapter == nil {
			return nil
		}
		for _, target := range task.Targets {
			if err := adapter.Send(context.Background(), target, resp.Text); err != nil {
				log.Printf("task %s send error: %v", task.ID, err)
			}
		}
	}

	return nil
}

func (s *Scheduler) AddTask(spec string, task func()) (cron.EntryID, error) {
	return s.cron.AddFunc(spec, task)
}

func (s *Scheduler) AddOneShot(delay time.Duration, task func()) {
	time.AfterFunc(delay, task)
}

// AddPersistentJob persists the job and schedules it
func (s *Scheduler) AddPersistentJob(task config.TaskConfig) error {
	if err := s.store.Add(task); err != nil {
		return err
	}
	return s.RegisterTasks([]config.TaskConfig{task})
}

// ListJobs returns a friendly list of scheduled jobs
func (s *Scheduler) ListJobs() ([]string, error) {
	tasks, err := s.store.List()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(tasks))
	for _, t := range tasks {
		desc := fmt.Sprintf("- %s: %s", t.ID, t.Cron)
		if len(t.Tools) > 0 {
			desc += fmt.Sprintf(" (tools: %d)", len(t.Tools))
		} else {
			desc += " (LLM)"
		}
		out = append(out, desc)
	}
	if len(out) == 0 {
		return []string{"No known scheduled jobs."}, nil
	}
	return out, nil
}
