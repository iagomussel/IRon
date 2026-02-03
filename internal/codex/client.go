package codex

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
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
	Text      string
	Stderr    string
	Code      int
	NewDir    string
	SessionID string
}

// Regex to capture directories from tool logs (e.g. "in /path/to/dir succeeded")
var dirRegex = regexp.MustCompile(`(?i)in\s+([~/][^\s]+)\s+succeeded`)

// Exec executes the codex command.
func (c *Client) Exec(ctx context.Context, sessionID string, cwd string, prompt string, useLast bool) (Response, error) {
	if len(c.Command) == 0 {
		return Response{}, errors.New("codex command not configured")
	}

	name := c.Command[0]
	args := c.prepareArgs(sessionID, useLast)

	log.Printf("codex exec: %s %s (cwd: %s, session: %s)", name, strings.Join(args, " "), cwd, sessionID)

	res, err := executil.Run(ctx, name, args, []byte(prompt), c.Env, c.Timeout, NormalizeCwd(cwd))

	stdout := res.Stdout
	stderr := res.Stderr

	// Try to discover if the agent changed directory by analyzing the logs
	newDir := cwd
	// Try to extract Session ID
	newSessionID := sessionID

	// We search both stdout and stderr (where tool logs usually go)
	combinedOutput := stdout + "\n" + stderr

	// Dir regex
	matches := dirRegex.FindAllStringSubmatch(combinedOutput, -1)
	if len(matches) > 0 {
		// Gets the last directory mentioned as "succeeded"
		lastMatch := matches[len(matches)-1][1]
		newDir = strings.TrimRight(lastMatch, ".:,") // Clean punctuation
	}

	// Session ID regex
	// Example: "session id: 019c24a1-925c-75a1-8bc0-ac9dcff8fbc3"
	sessionRegex := regexp.MustCompile(`(?i)session\s+id:\s+([0-9a-fA-F-]+)`)
	sMatches := sessionRegex.FindAllStringSubmatch(combinedOutput, -1)
	if len(sMatches) > 0 {
		newSessionID = sMatches[len(sMatches)-1][1]
	}

	stdoutClean := strings.TrimSpace(stdout)
	stderrClean := strings.TrimSpace(stderr)

	if stdoutClean != "" {
		log.Printf("codex stdout: %s", stdoutClean)
	}
	if stderrClean != "" {
		log.Printf("codex stderr: %s", stderrClean)
	}

	return Response{
		Text:      stdoutClean,
		Stderr:    stderrClean,
		Code:      res.Code,
		NewDir:    newDir,
		SessionID: newSessionID,
	}, err
}

func (c *Client) prepareArgs(sessionID string, useLast bool) []string {
	baseArgs := make([]string, 0, len(c.Command))
	for _, arg := range c.Command {
		if arg == "{session}" || arg == "resume" || arg == "--last" {
			continue
		}
		baseArgs = append(baseArgs, arg)
	}

	// Prepare resume arguments
	var resumeArgs []string
	if sessionID != "" {
		resumeArgs = []string{"resume", sessionID}
	} else if useLast {
		resumeArgs = []string{"resume", "--last"}
	} else {
		return baseArgs // No resume, start new
	}

	// Inject resume args before "-" (stdin prompt) if present, or append
	for i, arg := range baseArgs {
		if arg == "-" {
			out := make([]string, 0, len(baseArgs)+len(resumeArgs))
			out = append(out, baseArgs[:i]...)
			out = append(out, resumeArgs...)
			out = append(out, baseArgs[i:]...)
			return out
		}
	}
	return append(baseArgs, resumeArgs...)
}

func NormalizeCwd(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	home, _ := os.UserHomeDir()

	var finalPath string
	if cwd == "" || cwd == "~" || cwd == "~/" {
		finalPath = home
	} else if strings.HasPrefix(cwd, "~/") {
		if home != "" {
			finalPath = filepath.Join(home, strings.TrimPrefix(cwd, "~/"))
		} else {
			finalPath = cwd
		}
	} else {
		finalPath = cwd
	}

	// If directory doesn't exist, fallback to home to prevent session lock
	if _, err := os.Stat(finalPath); os.IsNotExist(err) {
		log.Printf("warning: directory %s does not exist, falling back to %s", finalPath, home)
		return home
	}
	return finalPath
}
