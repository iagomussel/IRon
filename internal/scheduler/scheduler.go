package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"agentic/internal/adapters"
	"agentic/internal/codex"
	"agentic/internal/config"
	"agentic/internal/db"
	"agentic/internal/ir"
	"agentic/internal/tools"

	"github.com/robfig/cron/v3"
)

type JobStore interface {
	Add(task config.TaskConfig) error
	List() ([]config.TaskConfig, error)
}

type SQLiteJobStore struct {
	db *db.DB
}

func NewSQLiteJobStore(d *db.DB) *SQLiteJobStore {
	return &SQLiteJobStore{db: d}
}

func (s *SQLiteJobStore) Add(task config.TaskConfig) error {
	toolsJSON, _ := json.Marshal(task.Tools)
	// We don't have a description field in TaskConfig yet, defaulting to ""
	return s.db.AddJob(task.ID, task.Cron, string(toolsJSON), task.Prompt, task.Adapter, task.Targets[0], "")
}

func (s *SQLiteJobStore) List() ([]config.TaskConfig, error) {
	jobs, err := s.db.ListJobs()
	if err != nil {
		return nil, err
	}
	var tasks []config.TaskConfig
	for _, j := range jobs {
		var toolsReq []ir.ToolRequest
		_ = json.Unmarshal([]byte(j.ToolsJSON), &toolsReq)
		tasks = append(tasks, config.TaskConfig{
			ID:      j.ID,
			Cron:    j.Cron,
			Tools:   toolsReq,
			Prompt:  j.Prompt,
			Adapter: j.Adapter,
			Targets: []string{j.Target},
		})
	}
	return tasks, nil
}

type Scheduler struct {
	cron     *cron.Cron
	codex    *codex.Client
	adapters *adapters.Registry
	tools    *tools.Registry
	store    JobStore

	mu         sync.Mutex
	memCron    map[cron.EntryID]string
	memOneShot map[string]string
}

func New(codexClient *codex.Client, adaptersReg *adapters.Registry, toolsReg *tools.Registry, database *db.DB) *Scheduler {
	// Standard parser (Minute Hour Dom Month Dow)
	s := &Scheduler{
		cron:       cron.New(),
		codex:      codexClient,
		adapters:   adaptersReg,
		tools:      toolsReg,
		store:      NewSQLiteJobStore(database),
		memCron:    make(map[cron.EntryID]string),
		memOneShot: make(map[string]string),
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

func (s *Scheduler) AddTask(spec string, task func(), desc string) (cron.EntryID, error) {
	id, err := s.cron.AddFunc(spec, task)
	if err == nil {
		s.mu.Lock()
		s.memCron[id] = fmt.Sprintf("[%s] %s", spec, desc)
		s.mu.Unlock()
	}
	return id, err
}

func (s *Scheduler) AddOneShot(delay time.Duration, task func(), desc string) {
	id := fmt.Sprintf("oneshot-%d", time.Now().UnixNano())
	s.mu.Lock()
	s.memOneShot[id] = fmt.Sprintf("[in %s] %s", delay, desc)
	s.mu.Unlock()

	time.AfterFunc(delay, func() {
		task()
		s.mu.Lock()
		delete(s.memOneShot, id)
		s.mu.Unlock()
	})
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
	var out []string

	// 1. Persistent Tasks
	tasks, err := s.store.List()
	if err == nil {
		for _, t := range tasks {
			desc := fmt.Sprintf("- [Persistent] %s: %s", t.ID, t.Cron)
			out = append(out, desc)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 2. Memory Cron
	for id, desc := range s.memCron {
		if s.cron.Entry(id).Valid() {
			out = append(out, fmt.Sprintf("- [Cron] %s", desc))
		}
	}

	// 3. Memory OneShot
	for _, desc := range s.memOneShot {
		out = append(out, fmt.Sprintf("- [OneShot] %s", desc))
	}

	if len(out) == 0 {
		return []string{"No known scheduled jobs."}, nil
	}
	return out, nil
}
