package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agentic/internal/config"
	"agentic/internal/ir"
	"agentic/internal/tools"
)

type Tool struct {
	scheduler *Scheduler
}

func NewTool(s *Scheduler) *Tool {
	return &Tool{scheduler: s}
}

func (t *Tool) Name() string {
	return "schedule"
}

type Input struct {
	Spec    string `json:"spec"`    // Date time (RFC3339), Duration (e.g. 30m), or Cron
	Message string `json:"message"` // Content to send
	Adapter string `json:"adapter"` // Adapter name, e.g. "telegram"
	Target  string `json:"target"`  // Target ID (e.g. ChatID)
}

func (t *Tool) Run(ctx context.Context, input json.RawMessage) (tools.Result, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return tools.Result{Error: err.Error()}, err
	}

	if in.Message == "" {
		return tools.Result{Error: "message is required"}, fmt.Errorf("message is required")
	}
	if in.Target == "" {
		return tools.Result{Error: "target is required"}, fmt.Errorf("target is required")
	}
	if in.Adapter == "" {
		in.Adapter = "telegram"
	}

	// Try duration (e.g. "30m")
	if d, err := time.ParseDuration(in.Spec); err == nil {
		t.scheduler.AddOneShot(d, func() {
			if adp := t.scheduler.adapters.Get(in.Adapter); adp != nil {
				_ = adp.Send(context.Background(), in.Target, in.Message)
			}
		})
		return tools.Result{Output: fmt.Sprintf("Scheduled one-shot task in %s", d)}, nil
	}

	// Try RFC3339
	if ts, err := time.Parse(time.RFC3339, in.Spec); err == nil {
		d := time.Until(ts)
		if d < 0 {
			return tools.Result{Error: "scheduled time is in the past"}, fmt.Errorf("past time")
		}
		t.scheduler.AddOneShot(d, func() {
			if adp := t.scheduler.adapters.Get(in.Adapter); adp != nil {
				_ = adp.Send(context.Background(), in.Target, in.Message)
			}
		})
		return tools.Result{Output: fmt.Sprintf("Scheduled one-shot task at %s", ts)}, nil
	}

	// Fallback to Cron
	_, err := t.scheduler.AddTask(in.Spec, func() {
		if adp := t.scheduler.adapters.Get(in.Adapter); adp != nil {
			_ = adp.Send(context.Background(), in.Target, in.Message)
		}
	})
	if err != nil {
		return tools.Result{Error: "invalid schedule spec: " + err.Error()}, err
	}

	return tools.Result{Output: fmt.Sprintf("Scheduled recurring task: %s", in.Spec)}, nil
}

// New Job DSL Tool

type ScheduleJobTool struct {
	scheduler *Scheduler
}

func NewScheduleJobTool(s *Scheduler) *ScheduleJobTool {
	return &ScheduleJobTool{scheduler: s}
}

func (t *ScheduleJobTool) Name() string { return "schedule_job" }

type JobInput struct {
	Name    string           `json:"name"`
	Cron    string           `json:"cron"`
	Tools   []ir.ToolRequest `json:"tools"`
	Adapter string           `json:"adapter"`
	Target  string           `json:"target"`
}

func (t *ScheduleJobTool) Run(ctx context.Context, input json.RawMessage) (tools.Result, error) {
	var in JobInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tools.Result{Error: err.Error()}, err
	}

	if in.Name == "" {
		return tools.Result{Error: "name is required"}, fmt.Errorf("name is required")
	}
	if in.Cron == "" {
		return tools.Result{Error: "cron is required"}, fmt.Errorf("cron is required")
	}
	if len(in.Tools) == 0 {
		return tools.Result{Error: "tools are required"}, fmt.Errorf("tools are required")
	}
	if in.Adapter == "" {
		in.Adapter = "telegram"
	}
	if in.Target == "" {
		// Try to infer target from context?
		// Ideally pass target in args, but robust code checks.
		// For now fail.
		return tools.Result{Error: "target is required"}, fmt.Errorf("target is required")
	}

	task := config.TaskConfig{
		ID:      in.Name,
		Cron:    in.Cron,
		Tools:   in.Tools,
		Adapter: in.Adapter,
		Targets: []string{in.Target},
	}

	if err := t.scheduler.AddPersistentJob(task); err != nil {
		return tools.Result{Error: "failed to schedule job: " + err.Error()}, err
	}

	return tools.Result{Output: fmt.Sprintf("Job '%s' scheduled @ %s", in.Name, in.Cron)}, nil
}
