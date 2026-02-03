package telegram

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"agentic/internal/adapters"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Adapter struct {
	bot          *tgbotapi.BotAPI
	allowedChat  map[int64]bool
	maxChunkSize int
}

func NewAdapter(token string, allowed []int64, maxChunkSize int) (*Adapter, error) {
	if token == "" {
		return nil, errors.New("telegram token is required")
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	allow := map[int64]bool{}
	for _, id := range allowed {
		allow[id] = true
	}
	if maxChunkSize <= 0 {
		maxChunkSize = 3500
	}
	return &Adapter{bot: bot, allowedChat: allow, maxChunkSize: maxChunkSize}, nil
}

func (a *Adapter) ID() string { return "telegram" }

func (a *Adapter) Start(ctx context.Context, onMessage func(adapters.Message)) error {
	update := tgbotapi.NewUpdate(0)
	update.Timeout = 60
	updates := a.bot.GetUpdatesChan(update)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case u := <-updates:
				if u.Message == nil {
					continue
				}
				chatID := u.Message.Chat.ID
				if len(a.allowedChat) > 0 && !a.allowedChat[chatID] {
					continue
				}
				onMessage(adapters.Message{
					SenderID: strconv.FormatInt(chatID, 10),
					Text:     strings.TrimSpace(u.Message.Text),
				})
			}
		}
	}()
	return nil
}

func (a *Adapter) Send(ctx context.Context, target string, text string) error {
	chatID, err := strconv.ParseInt(target, 10, 64)
	if err != nil {
		return err
	}
	chunks := chunkText(text, a.maxChunkSize)
	for _, chunk := range chunks {
		msg := tgbotapi.NewMessage(chatID, chunk)
		msg.ParseMode = ""
		msg.DisableWebPagePreview = true
		msg.ReplyMarkup = nil
		_, err := a.bot.Send(msg)
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return nil
}

func (a *Adapter) SendTyping(ctx context.Context, target string) error {
	chatID, err := strconv.ParseInt(target, 10, 64)
	if err != nil {
		return err
	}
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	_, err = a.bot.Send(action)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func chunkText(text string, size int) []string {
	if len(text) <= size {
		return []string{text}
	}
	out := []string{}
	for len(text) > size {
		out = append(out, text[:size])
		text = text[size:]
	}
	if text != "" {
		out = append(out, text)
	}
	return out
}
