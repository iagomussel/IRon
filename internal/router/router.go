package router

import (
	"encoding/json"
	"strings"

	"agentic/internal/ir"
)

type Router struct{}

func New() *Router {
	return &Router{}
}

// Route attempts to deterministically map input text to an IR Packet.
// Returns a Packet and true if a match is found with high confidence.
func (r *Router) Route(text string) (*ir.Packet, bool) {
	text = strings.TrimSpace(text)
	lower := strings.ToLower(text)

	// Help command
	if lower == "/help" || lower == "help" {
		return &ir.Packet{
			Action:     ir.ActionActNow,
			Intent:     "help",
			Risk:       ir.RiskNone,
			Confidence: 1.0,
			Tools:      []ir.ToolRequest{{Name: "help", Args: json.RawMessage(`{}`)}},
		}, true
	}

	// Simple reminder detection (e.g. "lembre-me em 10m de ...")
	// This is a basic example; a real router might use more complex DSL or parsing.
	// For now, we'll let complex scheduling go to the LLM, but catch very specific formats if needed.
	// Example: "ping" -> "pong"
	if lower == "ping" {
		return &ir.Packet{
			Action:     ir.ActionActNow,
			Intent:     "ping",
			Risk:       ir.RiskNone,
			Confidence: 1.0,
			Tools:      nil, // No tool, just reply
		}, true
	}

	// Logic for specific "deterministic" commands can be added here.
	// For instance, if the user starts with "note:" or "nota:", we map to notes_tool.
	if strings.HasPrefix(lower, "nota: ") || strings.HasPrefix(lower, "note: ") {
		content := strings.TrimPrefix(text[5:], " ") // remove prefix
		args, _ := json.Marshal(map[string]interface{}{
			"content": content,
		})
		return &ir.Packet{
			Action:     ir.ActionActNow,
			Intent:     "notes.append",
			Risk:       ir.RiskLow,
			Confidence: 1.0,
			Tools: []ir.ToolRequest{
				{Name: "notes_append", Args: args},
			},
		}, true
	}

	return nil, false
}

// GenerateReply creates a fallback reply for deterministic routes
func (r *Router) GenerateReply(packet *ir.Packet) string {
	switch packet.Intent {
	case "help":
		return "Available commands: note: <text>, ping, or speak naturally."
	case "ping":
		return "Pong!"
	case "notes.append":
		return "Note saved."
	default:
		return "Command processed."
	}
}
