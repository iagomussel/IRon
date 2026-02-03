package scheduler

import (
	"context"
	"log"

	"agentic/internal/adapters"
	"agentic/internal/codex"
	"agentic/internal/config"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron     *cron.Cron
	codex    *codex.Client
	adapters *adapters.Registry
}

func New(codexClient *codex.Client, adaptersReg *adapters.Registry) *Scheduler {
	return &Scheduler{
		cron:     cron.New(),
		codex:    codexClient,
		adapters: adaptersReg,
	}
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
