package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/mux"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"

	"github.com/yuya-takeyama/cc-slack/internal/mcp"
	"github.com/yuya-takeyama/cc-slack/internal/session"
	"github.com/yuya-takeyama/cc-slack/pkg/config"
)

type Bot struct {
	config          *config.Config
	client          *slack.Client
	sessionManager  *session.Manager
	approvalManager *mcp.ApprovalManager
	server          *http.Server
	mu              sync.RWMutex
}

func NewBot(cfg *config.Config) *Bot {
	client := slack.New(cfg.SlackBotToken)
	sessionManager := session.NewManager()
	approvalManager := mcp.NewApprovalManager(client)

	return &Bot{
		config:          cfg,
		client:          client,
		sessionManager:  sessionManager,
		approvalManager: approvalManager,
	}
}

func (b *Bot) GetApprovalManager() *mcp.ApprovalManager {
	return b.approvalManager
}

func (b *Bot) Start(ctx context.Context) error {
	router := mux.NewRouter()
	
	// Event API endpoint
	router.HandleFunc("/slack/events", b.handleSlackEvents).Methods("POST")
	
	// Interactive endpoint for button clicks
	router.HandleFunc("/slack/interactive", b.handleSlackInteractive).Methods("POST")
	
	// Health check endpoint
	router.HandleFunc("/health", b.handleHealth).Methods("GET")

	b.server = &http.Server{
		Addr:    ":" + strconv.Itoa(b.config.Port),
		Handler: router,
	}

	slog.Info("Starting Slack bot HTTP server", "port", b.config.Port)

	// Start server in goroutine
	go func() {
		if err := b.server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	
	slog.Info("Shutting down Slack bot HTTP server")
	return b.server.Shutdown(context.Background())
}

func (b *Bot) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (b *Bot) handleSlackInteractive(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		slog.Error("Failed to parse form", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	payload := r.FormValue("payload")
	if payload == "" {
		slog.Error("No payload in interactive request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var interaction slack.InteractionCallback
	if err := json.Unmarshal([]byte(payload), &interaction); err != nil {
		slog.Error("Failed to unmarshal interaction payload", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Handle approval interactions
	if err := b.approvalManager.HandleInteraction(interaction); err != nil {
		slog.Error("Failed to handle approval interaction", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (b *Bot) handleSlackEvents(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("Failed to read request body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	eventsAPIEvent, err := slackevents.ParseEvent(
		json.RawMessage(body),
		slackevents.OptionVerifyToken(&slackevents.TokenComparator{
			VerificationToken: b.config.SlackSigningSecret,
		}),
	)
	if err != nil {
		slog.Error("Failed to parse Slack event", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch eventsAPIEvent.Type {
	case slackevents.URLVerification:
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal(body, &r)
		if err != nil {
			slog.Error("Failed to unmarshal challenge", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text")
		w.Write([]byte(r.Challenge))

	case slackevents.CallbackEvent:
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			b.handleAppMention(ev)
		case *slackevents.MessageEvent:
			b.handleMessage(ev)
		}
		w.WriteHeader(http.StatusOK)

	default:
		slog.Warn("Unknown event type", "type", eventsAPIEvent.Type)
		w.WriteHeader(http.StatusOK)
	}
}

func (b *Bot) handleAppMention(event *slackevents.AppMentionEvent) {
	slog.Info("Received app mention", 
		"channel", event.Channel, 
		"user", event.User, 
		"text", event.Text,
		"thread_ts", event.ThreadTimeStamp)

	// Determine working directory
	workDir := b.determineWorkDir(event.Channel)

	// Check if this is in a thread (continue existing session)
	if event.ThreadTimeStamp != "" {
		session := b.sessionManager.GetByThreadTS(event.ThreadTimeStamp)
		if session != nil {
			// Send message to existing Claude Code process
			if err := b.sendToClaudeProcess(session, event.Text); err != nil {
				slog.Error("Failed to send to Claude process", "error", err)
				b.postErrorMessage(event.Channel, event.ThreadTimeStamp, "プロセスへの送信に失敗しました")
			}
			return
		}
	}

	// Start new Claude Code session with process
	session, err := b.createSessionWithProcess(event.Channel, workDir)
	if err != nil {
		slog.Error("Failed to create session", "error", err)
		b.postErrorMessage(event.Channel, event.ThreadTimeStamp, "セッションの作成に失敗しました")
		return
	}

	// Post initial response to get thread timestamp
	initialMsg := "🚀 Claude Code セッションを開始しています..."
	timestamp, err := b.postMessage(event.Channel, initialMsg, event.ThreadTimeStamp)
	if err != nil {
		slog.Error("Failed to post initial message", "error", err)
		return
	}

	// Update session with thread timestamp
	session.ThreadTS = timestamp
	b.sessionManager.UpdateThreadTS(session.SessionID, timestamp)

	// Start processing Claude Code output
	go b.processClaudeOutput(session)

	// Send user message to Claude Code process
	if err := b.sendToClaudeProcess(session, event.Text); err != nil {
		slog.Error("Failed to send initial message to Claude", "error", err)
		b.postErrorMessage(event.Channel, timestamp, "初期メッセージの送信に失敗しました")
	}
}

func (b *Bot) handleMessage(event *slackevents.MessageEvent) {
	// Handle threaded messages (additional instructions)
	if event.ThreadTimeStamp != "" && event.ThreadTimeStamp != event.TimeStamp {
		session := b.sessionManager.GetByThreadTS(event.ThreadTimeStamp)
		if session != nil {
			if err := b.sendToClaudeProcess(session, event.Text); err != nil {
				slog.Error("Failed to send threaded message to Claude", "error", err)
				b.postErrorMessage(event.Channel, event.ThreadTimeStamp, "メッセージの送信に失敗しました")
			}
		}
	}
}

func (b *Bot) determineWorkDir(channelID string) string {
	// TODO: Implement channel-specific work directory logic
	// For now, return default work directory
	return b.config.DefaultWorkDir
}

func (b *Bot) postMessage(channel, text, threadTS string) (string, error) {
	options := []slack.MsgOption{
		slack.MsgOptionText(text, false),
	}

	if threadTS != "" {
		options = append(options, slack.MsgOptionTS(threadTS))
	}

	_, timestamp, err := b.client.PostMessage(channel, options...)
	return timestamp, err
}

func (b *Bot) postErrorMessage(channel, threadTS, message string) {
	errorMsg := fmt.Sprintf("❌ エラー: %s", message)
	if _, err := b.postMessage(channel, errorMsg, threadTS); err != nil {
		slog.Error("Failed to post error message", "error", err)
	}
}

