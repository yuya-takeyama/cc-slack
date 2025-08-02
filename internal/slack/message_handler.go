package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/yuya-takeyama/cc-slack/internal/slack/blocks"
	"golang.org/x/sync/errgroup"
)

// HandleEvent handles Slack webhook events
func (h *Handler) HandleEvent(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	// Verify request signature
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

	// Parse event
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	// Handle URL verification
	if eventsAPIEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal(body, &r)
		if err != nil {
			http.Error(w, "Failed to parse challenge", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(r.Challenge))
		return
	}

	// Handle callback events
	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			h.handleMessage(ev)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleMessage handles message events
func (h *Handler) handleMessage(event *slackevents.MessageEvent) {
	// Apply filtering
	if !h.shouldProcessMessage(event) {
		return
	}

	// Extract message text, optionally removing bot mention if it exists
	text := event.Text
	if h.config.Slack.MessageFilter.RequireMention {
		text = h.removeBotMention(text)
		if text == "" {
			return
		}
	}

	// Determine thread timestamp for session management
	threadTS := event.ThreadTimeStamp
	if threadTS == "" {
		threadTS = event.TimeStamp
	}

	// Check if mentioned in a thread with an existing session
	if event.ThreadTimeStamp != "" {
		h.handleThreadMessageEvent(event, text)
	} else {
		h.handleNewSessionFromMessage(event, text, threadTS)
	}
}

// handleThreadMessageEvent handles message events in existing threads
func (h *Handler) handleThreadMessageEvent(event *slackevents.MessageEvent, text string) {
	// Try to find existing session
	session, err := h.sessionMgr.GetSessionByThread(event.Channel, event.ThreadTimeStamp)
	if err == nil && session != nil {
		// Process attachments directly from the event
		var imagePaths []string
		var files []slack.File
		if event.Message != nil {
			files = event.Message.Files
		}

		if h.fileUploadEnabled && len(files) > 0 {
			imagePaths = h.processMessageAttachments(event, files)
		}

		// Add image paths to the prompt if any
		if len(imagePaths) > 0 {
			text = h.appendImagePaths(text, imagePaths)
		}

		// Existing session found - send message to it
		err = h.sessionMgr.SendMessage(session.SessionID, text)
		if err != nil {
			h.client.PostMessage(
				event.Channel,
				slack.MsgOptionText(fmt.Sprintf("Failed to send message: %v", err), false),
				slack.MsgOptionTS(event.ThreadTimeStamp),
			)
		}
		return
	}

	// No existing session found - create new session
	h.handleNewSessionFromMessage(event, text, event.ThreadTimeStamp)
}

// handleNewSessionFromMessage creates a new session or resumes one for message events
func (h *Handler) handleNewSessionFromMessage(event *slackevents.MessageEvent, text string, threadTS string) {
	// In multi-directory mode, validate working directory availability
	if !h.config.IsSingleDirectoryMode() {
		// For new threads, prevent mention-based start
		if event.ThreadTimeStamp == "" {
			// Post error message with guidance
			blocksSlice := blocks.MultiDirectoryError(h.config.Slack.SlashCommandName)

			_, _, err := h.client.PostMessage(
				event.Channel,
				slack.MsgOptionBlocks(blocksSlice...),
				slack.MsgOptionTS(threadTS),
			)
			if err != nil {
				fmt.Printf("Failed to post error message: %v\n", err)
			}
			return
		}

		// For existing threads, we'll let the session manager handle validation
		// It will check if the thread has a working directory stored
	}

	// Determine working directory
	workDir := h.determineWorkDir(event.Channel)

	// Process attachments if any, but defer actual download
	var hasImages bool
	var files []slack.File
	// Check files in Message field
	if event.Message != nil && len(event.Message.Files) > 0 {
		files = event.Message.Files
	}

	if h.fileUploadEnabled && len(files) > 0 {
		for _, file := range files {
			if strings.HasPrefix(file.Mimetype, "image/") {
				hasImages = true
				break
			}
		}
	}

	// Process images first if any
	var initialPrompt string = text
	if hasImages && len(files) > 0 {
		imagePaths := h.processMessageAttachments(event, files)
		if len(imagePaths) > 0 {
			// Append image paths to the initial prompt
			initialPrompt = h.appendImagePaths(text, imagePaths)
		}
	}

	// Create session with text including image paths
	ctx := context.Background()
	resumed, previousSessionID, err := h.sessionMgr.CreateSession(ctx, event.Channel, threadTS, workDir, initialPrompt)
	if err != nil {
		h.client.PostMessage(
			event.Channel,
			slack.MsgOptionText(fmt.Sprintf("Failed to create session: %v", err), false),
			slack.MsgOptionTS(threadTS),
		)
		return
	}

	// Post initial response based on whether session was resumed
	var initialMessage string
	if resumed {
		initialMessage = fmt.Sprintf("Resuming previous session `%s`...", previousSessionID)
	} else {
		initialMessage = "🚀 Starting Claude Code session..."
	}

	_, _, err = h.client.PostMessage(
		event.Channel,
		slack.MsgOptionText(initialMessage, false),
		slack.MsgOptionTS(threadTS),
	)
	if err != nil {
		fmt.Printf("Failed to post message: %v\n", err)
		return
	}

	// Image processing has already been done before session creation
}

// shouldProcessMessage filters message events based on configuration
func (h *Handler) shouldProcessMessage(event *slackevents.MessageEvent) bool {
	// If filtering is disabled, process all messages
	if !h.config.Slack.MessageFilter.Enabled {
		return true
	}

	// Skip bot messages
	if event.BotID != "" {
		return false
	}

	// Skip subtypes (edits, deletes, etc.) but allow file_share
	if event.SubType != "" && event.SubType != "file_share" {
		return false
	}

	// Check if bot mention is required
	if h.config.Slack.MessageFilter.RequireMention {
		if !h.containsBotMention(event.Text) {
			return false
		}
	}

	// Check include patterns
	if len(h.config.Slack.MessageFilter.IncludePatterns) > 0 {
		matched := false
		for _, pattern := range h.config.Slack.MessageFilter.IncludePatterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			if re.MatchString(event.Text) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check exclude patterns
	if len(h.config.Slack.MessageFilter.ExcludePatterns) > 0 {
		for _, pattern := range h.config.Slack.MessageFilter.ExcludePatterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			if re.MatchString(event.Text) {
				return false
			}
		}
	}

	return true
}

// containsBotMention checks if the text contains a mention of the bot
func (h *Handler) containsBotMention(text string) bool {
	if h.botUserID == "" {
		return false
	}
	return strings.Contains(text, fmt.Sprintf("<@%s>", h.botUserID))
}

// removeBotMention removes bot mention from message text
func (h *Handler) removeBotMention(text string) string {
	return RemoveBotMentionFromText(text, h.botUserID)
}

// RemoveBotMentionFromText removes bot mention from text (pure function)
func RemoveBotMentionFromText(text string, botUserID string) string {
	if botUserID == "" {
		return text
	}

	// Remove <@BOTID> pattern
	botMention := fmt.Sprintf("<@%s>", botUserID)
	text = strings.TrimSpace(text)

	// Handle mention at the beginning
	if strings.HasPrefix(text, botMention) {
		text = strings.TrimSpace(text[len(botMention):])
	}

	// Handle mention anywhere else in the text
	text = strings.ReplaceAll(text, botMention, "")

	return strings.TrimSpace(text)
}

// processMessageAttachments processes attachments directly from message event
func (h *Handler) processMessageAttachments(event *slackevents.MessageEvent, files []slack.File) []string {
	// Create session-specific directory structure
	// Format: images/{thread_ts}/{uuid}/
	sessionID := uuid.New().String()
	sessionDir := event.ThreadTimeStamp
	if sessionDir == "" {
		sessionDir = event.TimeStamp
	}
	sessionDir = strings.ReplaceAll(sessionDir, ".", "_")

	imageDir := filepath.Join(h.imagesDir, sessionDir, sessionID)
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		fmt.Printf("Failed to create image directory: %v\n", err)
		return nil
	}

	// Filter image files
	var imageFiles []slack.File
	for _, file := range files {
		if strings.HasPrefix(file.Mimetype, "image/") {
			imageFiles = append(imageFiles, file)
		}
	}

	if len(imageFiles) == 0 {
		return nil
	}

	// Result structure to maintain order
	type result struct {
		path string
		idx  int
	}

	resultChan := make(chan result, len(imageFiles))
	errorChan := make(chan error, len(imageFiles))

	// Create a worker pool with limited concurrency
	ctx := context.Background()
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(MaxConcurrentDownloads)

	// Start download goroutines
	for i, file := range imageFiles {
		i, file := i, file // Capture loop variables
		g.Go(func() error {
			imagePath, err := h.downloadAndSaveImage(slack.File{
				ID:                 file.ID,
				Name:               file.Name,
				Mimetype:           file.Mimetype,
				URLPrivate:         file.URLPrivate,
				URLPrivateDownload: file.URLPrivateDownload,
			}, imageDir)

			if err != nil {
				errorChan <- fmt.Errorf("failed to download %s: %w", file.Name, err)
				return nil // Don't fail the whole group
			}

			resultChan <- result{path: imagePath, idx: i}
			return nil
		})
	}

	// Wait for all downloads to complete
	if err := g.Wait(); err != nil {
		return nil
	}

	close(resultChan)
	close(errorChan)

	// Log any errors
	for err := range errorChan {
		fmt.Printf("Download error: %v\n", err)
	}

	// Collect results in order
	results := make([]string, 0, len(imageFiles))
	pathsByIdx := make(map[int]string)
	for res := range resultChan {
		pathsByIdx[res.idx] = res.path
	}

	// Sort by original order
	for i := 0; i < len(imageFiles); i++ {
		if path, ok := pathsByIdx[i]; ok {
			results = append(results, path)
		}
	}

	return results
}

// downloadAndSaveImage downloads a Slack file and saves it locally
func (h *Handler) downloadAndSaveImage(file slack.File, imageDir string) (string, error) {
	// Generate unique filename
	ext := filepath.Ext(file.Name)
	if ext == "" {
		ext = ".jpg" // Default extension
	}

	// Keep original filename if possible
	filename := file.Name
	if filename == "" || filename == "image"+ext {
		// Generate a meaningful name if original is generic
		filename = fmt.Sprintf("%s-%s%s", file.ID, time.Now().Format("20060102-150405"), ext)
	}

	filePath := filepath.Join(imageDir, filename)

	// Download the file
	var downloadURL string
	if file.URLPrivateDownload != "" {
		downloadURL = file.URLPrivateDownload
	} else if file.URLPrivate != "" {
		downloadURL = file.URLPrivate
	} else {
		return "", fmt.Errorf("no download URL available for file %s", file.Name)
	}

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", "Bearer "+h.botToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read error response body for debugging
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error response body: %s\n", string(body))
		return "", fmt.Errorf("failed to download file: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Check if we got HTML instead of an image
	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/html") {
		return "", fmt.Errorf("received HTML instead of image, likely authentication issue - missing files:read scope?")
	}

	// Save to file
	out, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	// Return absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return filePath, nil // Return relative path if abs fails
	}

	return absPath, nil
}

// appendImagePaths appends image paths to the prompt
func (h *Handler) appendImagePaths(text string, imagePaths []string) string {
	if len(imagePaths) == 0 {
		return text
	}

	var builder strings.Builder
	builder.WriteString(text)
	builder.WriteString("\n\n# Images attached with the message\n")

	for i, path := range imagePaths {
		builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, path))
	}

	builder.WriteString("\n**IMPORTANT: Please read and analyze these images as they are part of the context for this message. Consider their content when formulating your response.**")

	return builder.String()
}
