package scheduler

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"agentic/internal/adapters"
	"agentic/internal/codex"
	"agentic/internal/config"

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
	store    JobStore
}

func New(codexClient *codex.Client, adaptersReg *adapters.Registry, dataDir string) *Scheduler {
	// Standard parser (Minute Hour Dom Month Dow)
	s := &Scheduler{
		cron:     cron.New(),
		codex:    codexClient,
		adapters: adaptersReg,
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
	resp, err := s.codex.Exec(context.Background(), "", task.Prompt, true)
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
