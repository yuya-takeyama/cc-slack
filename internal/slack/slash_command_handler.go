package slack

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
)

// HandleSlashCommand handles Slack slash commands (e.g., /cc)
func (h *Handler) HandleSlashCommand(w http.ResponseWriter, r *http.Request) {
	// Verify request signature
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	sv, err := slack.NewSecretsVerifier(r.Header, h.signingSecret)
	if err != nil {
		http.Error(w, "Failed to create secrets verifier", http.StatusBadRequest)
		return
	}
	if _, err := sv.Write(body); err != nil {
		http.Error(w, "Failed to verify signature", http.StatusInternalServerError)
		return
	}
	if err := sv.Ensure(); err != nil {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse slash command
	cmd, err := slack.SlashCommandParse(r)
	if err != nil {
		http.Error(w, "Failed to parse slash command", http.StatusBadRequest)
		return
	}

	// Log the command for debugging
	log.Info().
		Str("command", cmd.Command).
		Str("text", cmd.Text).
		Str("user_id", cmd.UserID).
		Str("channel_id", cmd.ChannelID).
		Msg("received slash command")

	// Handle /cc command
	if cmd.Command == "/cc" {
		// Open modal asynchronously
		go h.openRepoModal(cmd.TriggerID, cmd.ChannelID, cmd.UserID, cmd.Text)

		// Return 200 immediately
		w.WriteHeader(http.StatusOK)
		return
	}

	// Unknown command
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Unknown command")
}
