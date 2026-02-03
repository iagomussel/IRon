package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"agentic/internal/executil"
)

type ExternalAdapter struct {
	AdapterID string
	Command   []string
	Timeout   time.Duration
}

func (a *ExternalAdapter) ID() string { return a.AdapterID }

func (a *ExternalAdapter) Start(ctx context.Context, onMessage func(Message)) error {
	// External adapters are push-only. They don't receive messages unless
	// their binary implements a webhook/polling mechanism independently.
	return nil
}

func (a *ExternalAdapter) Send(ctx context.Context, target string, text string) error {
	if len(a.Command) == 0 {
		return errors.New("command is required")
	}
	payload := map[string]string{"target": target, "text": text}
	data, _ := json.Marshal(payload)
	_, err := executil.Run(ctx, a.Command[0], a.Command[1:], data, nil, a.Timeout, "")
	return err
}
