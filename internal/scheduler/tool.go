package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
