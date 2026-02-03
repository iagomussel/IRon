package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"agentic/internal/adapters"
	"agentic/internal/addons"
	"agentic/internal/codex"
	"agentic/internal/config"
	"agentic/internal/db"
	"agentic/internal/executil"
	"agentic/internal/ir"
	"agentic/internal/router"
	"agentic/internal/scheduler"
	"agentic/internal/store"
	"agentic/internal/telegram"
	"agentic/internal/tools"
)

func main() {
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("config load: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sessionStore, err := store.NewSessionStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	codexClient := &codex.Client{
		Command: cfg.CodexCommand,
		Env:     cfg.CodexEnv,
		Timeout: 20 * time.Minute,
	}

	adapterRegistry := adapters.NewRegistry()
	if cfg.TelegramToken != "" {
		tg, err := telegram.NewAdapter(cfg.TelegramToken, cfg.AllowedChatIDs, cfg.MaxResponseSize)
		if err != nil {
			log.Fatalf("telegram: %v", err)
		}
		adapterRegistry.Register(tg)
	}

	toolRegistry := tools.DefaultRegistry()

	database, err := db.New(filepath.Join(cfg.DataDir, "agent.db"))
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	defer database.Close()

	sched := scheduler.New(codexClient, adapterRegistry, toolRegistry, database)
	if err := sched.RegisterTasks(cfg.Tasks); err != nil {
		log.Fatalf("scheduler: %v", err)
	}

	toolRegistry.Register(scheduler.NewTool(sched))
	toolRegistry.RegisterAlias("remind", "schedule")
	toolRegistry.RegisterAlias("timer", "schedule")

	toolRegistry.Register(scheduler.NewScheduleJobTool(sched))
	toolRegistry.RegisterAlias("cron", "schedule_job")
	toolRegistry.RegisterAlias("job", "schedule_job")
	toolRegistry.RegisterAlias("task", "schedule_job")

	toolRegistry.Register(scheduler.NewListRemindersTool(sched))
	toolRegistry.RegisterAlias("reminders", "list_reminders")
	toolRegistry.RegisterAlias("list_reminders", "list_reminders")

	toolRegistry.Register(tools.NewNotesTool(cfg.DataDir))
	toolRegistry.RegisterAlias("note", "notes_append")
	toolRegistry.RegisterAlias("notes", "notes_append")
	toolRegistry.RegisterAlias("write_note", "notes_append")

	toolRegistry.Register(&tools.NotesShowTool{DataDir: cfg.DataDir})
	toolRegistry.RegisterAlias("show_notes", "notes_show")
	toolRegistry.RegisterAlias("list_notes", "notes_show")

	toolRegistry.Register(&tools.NotesClearTool{DataDir: cfg.DataDir})
	toolRegistry.RegisterAlias("clear_notes", "notes_clear")

	toolRegistry.Register(&tools.ListAddTool{BaseDir: cfg.DataDir})
	toolRegistry.RegisterAlias("list", "list_add") // ambiguous but 'list' implies adding often? or showing? 'list' command usually handled by router. But for tool call, list_add is safer default for 'list'.
	toolRegistry.RegisterAlias("add_list", "list_add")

	toolRegistry.Register(&tools.ListRemoveTool{BaseDir: cfg.DataDir})
	toolRegistry.RegisterAlias("remove_list", "list_remove")

	toolRegistry.Register(&tools.ListShowTool{BaseDir: cfg.DataDir})
	toolRegistry.RegisterAlias("show_list", "list_show")
	toolRegistry.RegisterAlias("get_list", "list_show")

	toolRegistry.Register(&tools.ListListsTool{BaseDir: cfg.DataDir})
	toolRegistry.RegisterAlias("lists", "list_lists")

	addonMgr := addons.New("addons")
	if err := addonMgr.Load(ctx, cfg.Addons, toolRegistry, adapterRegistry); err != nil {
		log.Fatalf("addons: %v", err)
	}

	toolServer := &tools.Server{Registry: toolRegistry}
	httpSrv := &http.Server{Addr: cfg.ToolsAddr, Handler: toolServer.Routes()}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("tools server error: %v", err)
		}
	}()

	sched.Start()

	adapter := adapterRegistry.Get("telegram")
	if adapter == nil {
		log.Println("telegram adapter not configured; exiting")
		return
	}
	if err := adapter.Start(ctx, func(msg adapters.Message) {
		go handleMessage(ctx, msg, adapter, codexClient, toolRegistry, sessionStore, sched)
	}); err != nil {
		log.Fatalf("adapter start: %v", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	_ = httpSrv.Shutdown(context.Background())
	_ = sched.Stop(context.Background())
}

func handleMessage(ctx context.Context, msg adapters.Message, adapter adapters.Adapter, codexClient *codex.Client, toolRegistry *tools.Registry, sessions *store.SessionStore, sched *scheduler.Scheduler) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	sessionKey := "telegram:" + msg.SenderID

	// Quick commands
	if text == "/new" {
		_ = sessionReset(ctx, sessions, sessionKey, adapter, msg.SenderID)
		return
	}
	if text == "/tools" {
		_ = adapter.Send(ctx, msg.SenderID, tools.FormatToolList(toolRegistry.List()))
		return
	}
	if text == "/help" {
		_ = adapter.Send(ctx, msg.SenderID, "Commands:\n/new - Reset session\n/cd <dir> - Change dir\n!cmd - Direct shell exec\n/tools - List tools")
		return
	}

	state, _ := sessions.GetState(sessionKey)
	currentDir := codex.NormalizeCwd(state.Dir)

	// Direct Shell Execution (!)
	if strings.HasPrefix(text, "!") {
		handleShell(ctx, text, currentDir, adapter, msg.SenderID, sessions, sessionKey, state.Dir)
		return
	}

	// Change Directory (/cd)
	if dir, rest, ok := parseDirCommand(text); ok {
		_ = sessions.SetDir(sessionKey, dir)
		if rest == "" {
			_ = adapter.Send(ctx, msg.SenderID, "Directory changed to: "+dir)
			return
		}
		text = rest
	}

	// 1. ROUTER: Deterministic check
	r := router.New()
	if packet, ok := r.Route(text); ok {
		log.Printf("router match: %s", packet.Intent)
		reply := r.GenerateReply(packet)
		stopTyping := startTyping(ctx, adapter, msg.SenderID)
		_ = adapter.Send(ctx, msg.SenderID, reply)
		stopTyping()
		executePacket(ctx, packet, toolRegistry, adapter, msg.SenderID)
		return
	}

	// 2. LLM: Gateway
	useLast := state.UseLast
	promptContext := ""
	if !useLast {
		// Load system prompt + metadata
		if content, err := os.ReadFile("prompt.txt"); err == nil {
			meta := fmt.Sprintf("Current Time: %s\nUser Chat ID: %s\n\n", time.Now().Format(time.RFC3339), msg.SenderID)
			promptContext = string(content) + "\n\n" + meta
		}
	}

	fullPrompt := promptContext + text
	stopTyping := startTyping(ctx, adapter, msg.SenderID)
	resp, err := codexClient.Exec(ctx, state.ID, state.Dir, fullPrompt, useLast)
	stopTyping()
	if err != nil {
		_ = adapter.Send(ctx, msg.SenderID, "LLM Error: "+err.Error())
		return
	}

	// Update session state
	if resp.SessionID != "" && resp.SessionID != state.ID {
		_ = sessions.SetSessionID(sessionKey, resp.SessionID)
		// Update local state copy for potential immediate reuse (e.g. repair)
		state.ID = resp.SessionID
	}
	if resp.NewDir != "" && resp.NewDir != state.Dir {
		_ = sessions.SetDir(sessionKey, resp.NewDir)
	}
	_ = sessions.SetUseLast(sessionKey, true)

	// 3. PARSE & REPAIR
	agentResp, ok := parseResponse(ctx, codexClient, adapter, msg.SenderID, text, resp.Text, state.ID, state.Dir)
	if !ok {
		return
	}

	// 4. EXECUTION
	needProcess := processResponse(ctx, &agentResp, codexClient, adapter, msg.SenderID, toolRegistry, sched, state.ID, state.Dir)
	if !needProcess {
		return
	}

	for i := 0; i < 5; i++ {
		stopTyping := startTyping(ctx, adapter, msg.SenderID)
		nextResp, err := codexClient.Exec(ctx, state.ID, state.Dir, "continue", true)
		stopTyping()
		if err != nil {
			_ = adapter.Send(ctx, msg.SenderID, "LLM Error: "+err.Error())
			return
		}

		if nextResp.SessionID != "" && nextResp.SessionID != state.ID {
			_ = sessions.SetSessionID(sessionKey, nextResp.SessionID)
			state.ID = nextResp.SessionID
		}
		if nextResp.NewDir != "" && nextResp.NewDir != state.Dir {
			_ = sessions.SetDir(sessionKey, nextResp.NewDir)
			state.Dir = nextResp.NewDir
		}

		agentResp, ok = parseResponse(ctx, codexClient, adapter, msg.SenderID, "continue", nextResp.Text, state.ID, state.Dir)
		if !ok {
			return
		}

		if !processResponse(ctx, &agentResp, codexClient, adapter, msg.SenderID, toolRegistry, sched, state.ID, state.Dir) {
			return
		}
	}
}

func parseResponse(ctx context.Context, codexClient *codex.Client, adapter adapters.Adapter, senderID, prompt, raw, sessionID, dir string) (ir.Response, bool) {
	var agentResp ir.Response
	if err := json.Unmarshal([]byte(raw), &agentResp); err != nil {
		log.Printf("json parse error: %v. attempting repair...", err)
		repairPrompt := fmt.Sprintf(`System: You returned invalid JSON. Fix it strictly following the schema.
Input was: %s
Output was: %s
Error: %v
Return JSON only.`, prompt, raw, err)

		stopTyping := startTyping(ctx, adapter, senderID)
		repairResp, rErr := codexClient.Exec(ctx, sessionID, dir, repairPrompt, false)
		stopTyping()
		if rErr == nil {
			if err2 := json.Unmarshal([]byte(repairResp.Text), &agentResp); err2 == nil {
				log.Println("repair successful")
			} else {
				log.Printf("repair failed: %v", err2)
				_ = adapter.Send(ctx, senderID, raw)
				return ir.Response{}, false
			}
		} else {
			return ir.Response{}, false
		}
	}

	return agentResp, true
}

func processResponse(ctx context.Context, agentResp *ir.Response, codexClient *codex.Client, adapter adapters.Adapter, senderID string, toolRegistry *tools.Registry, sched *scheduler.Scheduler, sessionID, dir string) bool {
	if agentResp.Reply != "" {
		_ = adapter.Send(ctx, senderID, agentResp.Reply)
	}

	if agentResp.IR == nil {
		return agentResp.NeedProcess
	}

	if err := agentResp.IR.Validate(); err != nil {
		log.Printf("ir validation failed: %v. attempting repair...", err)
		repairPrompt := fmt.Sprintf(`System: IR validation failed: %v. 
You must fix the JSON. Allowed actions: act_now, schedule, ask, defer.
Return JSON only.`, err)

		stopTyping := startTyping(ctx, adapter, senderID)
		repairResp, rErr := codexClient.Exec(ctx, sessionID, dir, repairPrompt, false)
		stopTyping()
		if rErr != nil {
			log.Printf("semantic repair exec failed: %v", rErr)
			return false
		}
		if err2 := json.Unmarshal([]byte(repairResp.Text), agentResp); err2 != nil {
			log.Printf("semantic repair json parse failed: %v", err2)
			return false
		}
		if err3 := agentResp.IR.Validate(); err3 != nil {
			log.Printf("semantic repair failed: %v", err3)
			_ = adapter.Send(ctx, senderID, "Critical error: Agent produced invalid action twice.")
			return false
		}
	}

	if agentResp.IR.Action == ir.ActionListReminders {
		jobs, err := sched.ListJobs()
		if err != nil {
			_ = adapter.Send(ctx, senderID, "Error listing jobs: "+err.Error())
		} else {
			_ = adapter.Send(ctx, senderID, strings.Join(jobs, "\n"))
		}
		return agentResp.NeedProcess
	}

	executePacket(ctx, agentResp.IR, toolRegistry, adapter, senderID)
	return agentResp.NeedProcess
}

func startTyping(ctx context.Context, adapter adapters.Adapter, target string) func() {
	ta, ok := adapter.(adapters.TypingSender)
	if !ok {
		return func() {}
	}
	typingCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		_ = ta.SendTyping(typingCtx, target)
		for {
			select {
			case <-typingCtx.Done():
				return
			case <-ticker.C:
				_ = ta.SendTyping(typingCtx, target)
			}
		}
	}()
	return cancel
}

func executePacket(ctx context.Context, packet *ir.Packet, registry *tools.Registry, adapter adapters.Adapter, targetID string) {
	if len(packet.Tools) > 0 {
		_ = sendStatus(ctx, adapter, targetID, fmt.Sprintf("Status: iniciando %d tool(s)...", len(packet.Tools)))
	}
	type toolResult struct {
		name string
		err  error
	}
	results := make([]toolResult, 0, len(packet.Tools))
	for _, req := range packet.Tools {
		tool := registry.Get(req.Name)
		if tool == nil {
			log.Printf("tool not found: %s", req.Name)
			results = append(results, toolResult{name: req.Name, err: fmt.Errorf("tool not found")})
			continue
		}

		// Inject target if missing/needed for specific tools
		if req.Name == "schedule" || req.Name == "schedule_job" {
			var argsMap map[string]interface{}
			if err := json.Unmarshal(req.Args, &argsMap); err == nil {
				if _, ok := argsMap["target"]; !ok {
					argsMap["target"] = targetID
					if newArgs, err := json.Marshal(argsMap); err == nil {
						req.Args = newArgs
					}
				}
			}
		}

		res, err := tool.Run(ctx, req.Args)
		if err != nil {
			log.Printf("tool %s error: %v", req.Name, err)
			_ = adapter.Send(ctx, targetID, fmt.Sprintf("[System] Tool error %s: %v", req.Name, err))
		} else {
			log.Printf("tool %s success: %s", req.Name, res.Output)
			// Optionally notify user of success if verbose
		}
		results = append(results, toolResult{name: req.Name, err: err})
	}
	if len(results) == 0 {
		return
	}
	var okCount, failCount int
	details := make([]string, 0, len(results))
	for _, r := range results {
		if r.err != nil {
			failCount++
			details = append(details, fmt.Sprintf("%s falhou", r.name))
		} else {
			okCount++
			details = append(details, fmt.Sprintf("%s ok", r.name))
		}
	}
	summary := fmt.Sprintf("Status: concluído. Sucesso: %d, Falhas: %d.", okCount, failCount)
	if len(details) > 0 {
		summary += " Detalhes: " + strings.Join(details, "; ") + "."
	}
	_ = sendStatus(ctx, adapter, targetID, summary)
}

func sessionReset(ctx context.Context, s *store.SessionStore, key string, adapter adapters.Adapter, sender string) error {
	_ = s.SetUseLast(key, false)
	_ = s.SetDir(key, "")
	return adapter.Send(ctx, sender, "Session reset.")
}

func handleShell(ctx context.Context, text, currentDir string, adapter adapters.Adapter, sender string, s *store.SessionStore, key, stateDir string) {
	cmd := strings.TrimSpace(strings.TrimPrefix(text, "!"))
	if cmd == "" {
		return
	}

	res, err := executil.Run(ctx, "bash", []string{"-c", cmd}, nil, nil, 1*time.Minute, currentDir)
	if err != nil {
		log.Printf("shell exec error: %v", err)
	}
	output := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
	if output == "" {
		output = fmt.Sprintf("(code %d)", res.Code)
	}

	if strings.HasPrefix(cmd, "cd ") {
		resPwd, _ := executil.Run(ctx, "bash", []string{"-c", cmd + " && pwd"}, nil, nil, 5*time.Second, currentDir)
		newDir := strings.TrimSpace(resPwd.Stdout)
		if newDir != "" && newDir != stateDir {
			_ = s.SetDir(key, newDir)
			output += "\nwd: " + newDir
		}
	}
	_ = adapter.Send(ctx, sender, output)
}

func parseDirCommand(text string) (string, string, bool) {
	text = strings.TrimSpace(text)
	var raw string
	if strings.HasPrefix(text, "/cd ") {
		raw = strings.TrimSpace(strings.TrimPrefix(text, "/cd "))
	} else if strings.HasPrefix(text, "cd ") && !strings.Contains(text, "\n") {
		raw = strings.TrimSpace(strings.TrimPrefix(text, "cd "))
	} else {
		return "", "", false
	}

	if raw == "" {
		return "", "", true
	}

	dir := raw
	rest := ""
	if parts := strings.SplitN(raw, "&&", 2); len(parts) == 2 {
		dir = strings.TrimSpace(parts[0])
		rest = strings.TrimSpace(parts[1])
	}
	return dir, rest, true
}

func sendStatus(ctx context.Context, adapter adapters.Adapter, target string, text string) error {
	return adapter.Send(ctx, target, text)
}
