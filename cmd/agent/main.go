package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"agentic/internal/adapters"
	"agentic/internal/addons"
	"agentic/internal/codex"
	"agentic/internal/config"
	"agentic/internal/executil"
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

	toolRegistry := tools.DefaultRegistry()
	adapterRegistry := adapters.NewRegistry()

	sessionStore, err := store.NewSessionStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	if cfg.TelegramToken != "" {
		tg, err := telegram.NewAdapter(cfg.TelegramToken, cfg.AllowedChatIDs, cfg.MaxResponseSize)
		if err != nil {
			log.Fatalf("telegram: %v", err)
		}
		adapterRegistry.Register(tg)
	}

	addonMgr := addons.New("addons")
	if err := addonMgr.Load(ctx, cfg.Addons, toolRegistry, adapterRegistry); err != nil {
		log.Fatalf("addons: %v", err)
	}

	codexClient := &codex.Client{
		Command: cfg.CodexCommand,
		Env:     cfg.CodexEnv,
		Timeout: 20 * time.Minute,
	}

	toolServer := &tools.Server{Registry: toolRegistry}
	httpSrv := &http.Server{Addr: cfg.ToolsAddr, Handler: toolServer.Routes()}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("tools server error: %v", err)
		}
	}()

	sched := scheduler.New(codexClient, adapterRegistry)
	if err := sched.RegisterTasks(cfg.Tasks); err != nil {
		log.Fatalf("scheduler: %v", err)
	}
	sched.Start()

	adapter := adapterRegistry.Get("telegram")
	if adapter == nil {
		log.Println("telegram adapter not configured; exiting")
		return
	}
	if err := adapter.Start(ctx, func(msg adapters.Message) {
		handleMessage(ctx, msg, adapter, codexClient, toolRegistry, sessionStore)
	}); err != nil {
		log.Fatalf("adapter start: %v", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	_ = httpSrv.Shutdown(context.Background())
	_ = sched.Stop(context.Background())
}

func handleMessage(ctx context.Context, msg adapters.Message, adapter adapters.Adapter, codexClient *codex.Client, toolRegistry *tools.Registry, sessions *store.SessionStore) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	sessionKey := "telegram:" + msg.SenderID

	if text == "/new" {
		_ = sessions.SetUseLast(sessionKey, false)
		_ = sessions.SetDir(sessionKey, "")
		_ = adapter.Send(ctx, msg.SenderID, "Sessão reiniciada.")
		return
	}
	if text == "/tools" {
		_ = adapter.Send(ctx, msg.SenderID, tools.FormatToolList(toolRegistry.List()))
		return
	}
	if text == "/help" {
		_ = adapter.Send(ctx, msg.SenderID, "Comandos:\n/new - Limpa sessão\n/cd <dir> - Muda pasta\n!cmd - Executa shell direto\n/tools - Lista ferramentas\n/help - Ajuda")
		return
	}

	state, _ := sessions.GetState(sessionKey)
	currentDir := codex.NormalizeCwd(state.Dir) // Usando o normalizador robusto do codex

	// EXECUÇÃO DIRETA NO SHELL (Modo !)
	if strings.HasPrefix(text, "!") {
		cmd := strings.TrimSpace(strings.TrimPrefix(text, "!"))
		if cmd == "" {
			return
		}

		log.Printf("direct shell exec: %s (cwd: %s)", cmd, currentDir)
		res, err := executil.Run(ctx, "bash", []string{"-c", cmd}, nil, nil, 1*time.Minute, currentDir)

		log.Printf("exec result: code=%d err=%v stdout=%q stderr=%q", res.Code, err, res.Stdout, res.Stderr)

		if err != nil && res.Code == -1 {
			_ = adapter.Send(ctx, msg.SenderID, "Falha ao iniciar comando: "+err.Error())
			return
		}

		output := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
		if output == "" {
			output = fmt.Sprintf("(comando finalizado com código %d)", res.Code)
		}

		if strings.HasPrefix(cmd, "cd ") {
			resPwd, _ := executil.Run(ctx, "bash", []string{"-c", cmd + " && pwd"}, nil, nil, 5*time.Second, currentDir)
			newDir := strings.TrimSpace(resPwd.Stdout)
			if newDir != "" && newDir != state.Dir {
				_ = sessions.SetDir(sessionKey, newDir)
				output += "\n\nDiretório atualizado: " + newDir
			}
		}

		_ = adapter.Send(ctx, msg.SenderID, "```\n"+output+"\n```")
		return
	}

	if dir, rest, ok := parseDirCommand(text); ok {
		_ = sessions.SetDir(sessionKey, dir)
		if rest == "" {
			_ = adapter.Send(ctx, msg.SenderID, "Diretório alterado para: "+dir)
			return
		}
		text = rest
	}

	// EXECUÇÃO VIA CODEX
	useLast := true
	if !state.UseLast && state.ID == "" {
		useLast = false
	}

	if !useLast {
		if content, err := os.ReadFile("prompt.txt"); err == nil {
			text = string(content) + "\n\n" + text
			log.Println("system prompt injected")
		} else {
			log.Printf("failed to read prompt.txt: %v", err)
		}
	}

	resp, err := codexClient.Exec(ctx, state.Dir, text, useLast)
	if err != nil {
		_ = adapter.Send(ctx, msg.SenderID, "Erro ao executar codex: "+err.Error())
		return
	}

	if resp.NewDir != "" && resp.NewDir != state.Dir {
		_ = sessions.SetDir(sessionKey, resp.NewDir)
	}

	_ = sessions.SetUseLast(sessionKey, true)

	if resp.Text == "" {
		resp.Text = "(sem resposta)"
	}
	_ = adapter.Send(ctx, msg.SenderID, resp.Text)
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
