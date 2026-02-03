package ir

import (
	"encoding/json"
	"fmt"
)

// Action types
const (
	ActionActNow        = "act_now"
	ActionSchedule      = "schedule"
	ActionAsk           = "ask"
	ActionDefer         = "defer"
	ActionListReminders = "list_reminders"
)

// Risk levels
const (
	RiskNone   = "none"
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"
)

// ToolRequest represents a tool execution request
type ToolRequest struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// Packet represents the machine action object
type Packet struct {
	Action     string        `json:"action"`
	Intent     string        `json:"intent"`
	Risk       string        `json:"risk"`
	When       string        `json:"when,omitempty"` // RRULE or crontab
	Tools      []ToolRequest `json:"tools,omitempty"`
	Confidence float64       `json:"confidence"`
}

// Response represents the specific dual-output format for the LLM
type Response struct {
	Reply       string  `json:"reply"`       // Short human message
	NeedProcess bool    `json:"needProcees"` // Whether the agent should continue
	IR          *Packet `json:"ir"`          // Machine action
}

func (r *Response) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["reply"]; ok {
		_ = json.Unmarshal(v, &r.Reply)
	}
	if v, ok := raw["ir"]; ok {
		_ = json.Unmarshal(v, &r.IR)
	}
	if v, ok := raw["needProcees"]; ok {
		_ = json.Unmarshal(v, &r.NeedProcess)
	} else if v, ok := raw["needProcess"]; ok {
		_ = json.Unmarshal(v, &r.NeedProcess)
	}
	return nil
}

// Validate checks if the packet is valid
func (p *Packet) Validate() error {
	switch p.Action {
	case ActionActNow, ActionSchedule, ActionAsk, ActionDefer, ActionListReminders:
		// valid
	default:
		return fmt.Errorf("invalid action: %s", p.Action)
	}

	switch p.Risk {
	case RiskNone, RiskLow, RiskMedium, RiskHigh, "":
		// valid
	default:
		return fmt.Errorf("invalid risk: %s", p.Risk)
	}

	return nil
}
