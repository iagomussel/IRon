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
	// Note DSL: "note: content" or "nota: content"
	if strings.HasPrefix(lower, "nota:") || strings.HasPrefix(lower, "note:") ||
		strings.HasPrefix(lower, "nota ") || strings.HasPrefix(lower, "note ") {

		parts := strings.SplitN(text, ":", 2)
		var content string
		if len(parts) == 2 {
			content = strings.TrimSpace(parts[1])
		} else {
			// Handle space case "note algo"
			parts = strings.SplitN(text, " ", 2)
			if len(parts) == 2 {
				content = strings.TrimSpace(parts[1])
			}
		}

		if content != "" {
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
	}

	// List DSL: "list <bucket> += <item>"
	// Regex: ^list\s+(\w+)\s*(\+=|-=|\?)\s*(.*)$
	// We handle: += (add), -= (remove), ? (show)
	if strings.HasPrefix(lower, "list ") {
		// Manual parse to avoid heavy regex if possible, but regex is cleaner for mixed types
		// Let's use simple logic
		rest := text[5:] // "bucket += item"
		parts := strings.Fields(rest)
		if len(parts) >= 2 {
			bucket := parts[0]
			op := parts[1]

			// Reconstruct item (rest of string)
			// "list bucket += item one two"
			// fields: [bucket, +=, item, one, two]
			item := ""
			if len(parts) > 2 {
				// Find where op ends in original string to preserve whitespace in item?
				// Simple approach: join parts[2:]
				item = strings.Join(parts[2:], " ")
			}

			var toolName string
			var intent string

			switch op {
			case "+=":
				toolName = "list_add"
				intent = "list.add"
			case "-=":
				toolName = "list_remove"
				intent = "list.remove"
			case "?":
				toolName = "list_show"
				intent = "list.show"
			}

			if toolName != "" {
				args, _ := json.Marshal(map[string]interface{}{
					"list": bucket,
					"item": item,
				})
				return &ir.Packet{
					Action:     ir.ActionActNow,
					Intent:     intent,
					Risk:       ir.RiskLow,
					Confidence: 1.0,
					Tools: []ir.ToolRequest{
						{Name: toolName, Args: args},
					},
				}, true
			}
		}
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
