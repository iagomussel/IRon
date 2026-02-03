package codex

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agentic/internal/executil"
)

type Client struct {
	Command []string
	Env     []string
	Timeout time.Duration
}

type Response struct {
	Text   string
	Stderr string
	Code   int
}

// Exec executa o comando codex. O parâmetro useLast ativa o modo 'resume --last'
func (c *Client) Exec(ctx context.Context, cwd string, prompt string, useLast bool) (Response, error) {
	if len(c.Command) == 0 {
		return Response{}, errors.New("codex command not configured")
	}

	name := c.Command[0]
	args := c.prepareArgs(useLast)

	log.Printf("codex exec: %s %s", name, strings.Join(args, " "))

	res, err := executil.Run(ctx, name, args, []byte(prompt), c.Env, c.Timeout, normalizeCwd(cwd))

	stdout := strings.TrimSpace(res.Stdout)
	stderr := strings.TrimSpace(res.Stderr)

	if stdout != "" {
		log.Printf("codex stdout: %s", stdout)
	}
	if stderr != "" {
		log.Printf("codex stderr: %s", stderr)
	}

	return Response{Text: stdout, Stderr: stderr, Code: res.Code}, err
}

func (c *Client) prepareArgs(useLast bool) []string {
	// Filtra placeholders antigos e prepara a base dos argumentos
	baseArgs := make([]string, 0, len(c.Command)-1)
	for _, arg := range c.Command[1:] {
		if arg == "{session}" || arg == "resume" || arg == "--last" {
			continue
		}
		baseArgs = append(baseArgs, arg)
	}

	if !useLast {
		return baseArgs
	}

	// Injeta 'resume --last' antes do marcador de stdin '-' ou no final
	for i, arg := range baseArgs {
		if arg == "-" {
			out := make([]string, 0, len(baseArgs)+2)
			out = append(out, baseArgs[:i]...)
			out = append(out, "resume", "--last")
			out = append(out, baseArgs[i:]...)
			return out
		}
	}
	return append(baseArgs, "resume", "--last")
}

func normalizeCwd(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if !strings.HasPrefix(cwd, "~") {
		return cwd
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return cwd
	}
	if cwd == "~" {
		return home
	}
	return filepath.Join(home, strings.TrimPrefix(cwd, "~/"))
}
