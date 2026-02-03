package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
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

func (t *Tool) Description() string {
	return "Schedule a one-off reminder or message. Args: spec (duration/rfc3339), message, target."
}

type Input struct {
	Spec    string `json:"spec"`    // Date time (RFC3339), Duration (e.g. 30m), or Cron
	When    string `json:"when"`    // Alias for Spec, often hallucinated by LLM
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

	// Normalize Spec/When
	if (in.Spec == "" || in.Spec == "once") && in.When != "" {
		in.Spec = in.When
	}

	// Try duration (e.g. "30m")
	if d, err := time.ParseDuration(in.Spec); err == nil {
		t.scheduler.AddOneShot(d, func() {
			log.Printf("executing one-shot schedule: message='%s' target='%s' adapter='%s'", in.Message, in.Target, in.Adapter)
			adp := t.scheduler.adapters.Get(in.Adapter)
			if adp == nil {
				log.Printf("error: adapter '%s' not found", in.Adapter)
				return
			}
			msg := strings.ReplaceAll(in.Message, "{{time}}", time.Now().Format("15:04:05"))
			msg = strings.ReplaceAll(msg, "{{date}}", time.Now().Format("2006-01-02"))

			if err := adp.Send(context.Background(), in.Target, msg); err != nil {
				log.Printf("error sending scheduled message: %v", err)
			}
		}, in.Message)
		return tools.Result{Output: fmt.Sprintf("Scheduled one-shot task in %s", d)}, nil
	}

	// Try RFC3339
	if ts, err := time.Parse(time.RFC3339, in.Spec); err == nil {
		d := time.Until(ts)
		note := ""
		if d < 0 {
			// Instead of failing, we execute immediately if it's in the past
			// This handles LLM clock drift or slight delays
			log.Printf("warning: scheduled time %s is in the past (%s). executing immediately.", in.Spec, d)
			d = 0
			note = " (time was in past, executing now)"
		}
		t.scheduler.AddOneShot(d, func() {
			log.Printf("executing one-shot schedule (rfc3339): message='%s' target='%s'", in.Message, in.Target)
			adp := t.scheduler.adapters.Get(in.Adapter)
			if adp == nil {
				log.Printf("error: adapter '%s' not found", in.Adapter)
				return
			}
			msg := strings.ReplaceAll(in.Message, "{{time}}", time.Now().Format("15:04:05"))
			msg = strings.ReplaceAll(msg, "{{date}}", time.Now().Format("2006-01-02"))

			if err := adp.Send(context.Background(), in.Target, msg); err != nil {
				log.Printf("error sending scheduled message: %v", err)
			}
		}, in.Message)
		return tools.Result{Output: fmt.Sprintf("Scheduled one-shot task at %s%s", ts, note)}, nil
	}

	// Fallback to Cron
	_, err := t.scheduler.AddTask(in.Spec, func() {
		log.Printf("executing cron schedule: message='%s' target='%s'", in.Message, in.Target)
		adp := t.scheduler.adapters.Get(in.Adapter)
		if adp == nil {
			log.Printf("error: adapter '%s' not found", in.Adapter)
			return
		}
		msg := strings.ReplaceAll(in.Message, "{{time}}", time.Now().Format("15:04:05"))
		msg = strings.ReplaceAll(msg, "{{date}}", time.Now().Format("2006-01-02"))

		if err := adp.Send(context.Background(), in.Target, msg); err != nil {
			log.Printf("error sending scheduled message: %v", err)
		}
	}, in.Message)
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

func (t *ScheduleJobTool) Description() string {
	return "Schedule a recurring job or complex task (cron). Modes: Tool-only, LLM-only, Hybrid."
}

type JobInput struct {
	Name    string           `json:"name"`
	Cron    string           `json:"cron"`
	Tools   []ir.ToolRequest `json:"tools"`
	Prompt  string           `json:"prompt"` // Optional: if present, runs tools then feeds output to LLM
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
	// Tools or Prompt must be present. If both, it's hybrid.
	if len(in.Tools) == 0 && in.Prompt == "" {
		return tools.Result{Error: "either tools or prompt (or both) are required"}, fmt.Errorf("missing tools/prompt")
	}
	if in.Adapter == "" {
		in.Adapter = "telegram"
	}
	if in.Target == "" {
		return tools.Result{Error: "target is required"}, fmt.Errorf("target is required")
	}

	task := config.TaskConfig{
		ID:      in.Name,
		Cron:    in.Cron,
		Tools:   in.Tools,
		Prompt:  in.Prompt,
		Adapter: in.Adapter,
		Targets: []string{in.Target},
	}

	if err := t.scheduler.AddPersistentJob(task); err != nil {
		return tools.Result{Error: "failed to schedule job: " + err.Error()}, err
	}

	return tools.Result{Output: fmt.Sprintf("Job '%s' scheduled @ %s", in.Name, in.Cron)}, nil
}
