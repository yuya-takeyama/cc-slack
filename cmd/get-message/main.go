package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/slack-go/slack"
)

func main() {
	// Slack APIトークンを環境変数から取得
	token := os.Getenv("CC_SLACK_SLACK_BOT_TOKEN")
	if token == "" {
		log.Fatal("CC_SLACK_SLACK_BOT_TOKEN environment variable is not set")
	}

	// チャンネルIDとタイムスタンプをコマンドライン引数から取得
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <channel_id> <timestamp>")
		fmt.Println("Example: go run main.go C12345678 1234567890.123456")
		os.Exit(1)
	}

	channelID := os.Args[1]
	timestamp := os.Args[2]

	// Slack APIクライアントを作成
	api := slack.New(token)

	// conversations.history APIを使ってメッセージを取得
	params := &slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Latest:    timestamp,
		Inclusive: true,
		Limit:     1,
	}

	history, err := api.GetConversationHistory(params)
	if err != nil {
		log.Fatalf("Failed to get conversation history: %v", err)
	}

	if len(history.Messages) == 0 {
		fmt.Println("No messages found")
		return
	}

	// メッセージを取得
	msg := history.Messages[0]

	// メッセージの詳細情報を表示
	fmt.Printf("=== Message Details ===\n")
	fmt.Printf("Text: %s\n", msg.Text)
	fmt.Printf("User: %s\n", msg.User)
	fmt.Printf("Timestamp: %s\n", msg.Timestamp)
	fmt.Printf("ThreadTimestamp: %s\n", msg.ThreadTimestamp)
	fmt.Printf("\n")

	// 添付ファイル情報を表示
	if len(msg.Files) > 0 {
		fmt.Printf("=== Files (%d) ===\n", len(msg.Files))
		for i, file := range msg.Files {
			fmt.Printf("\nFile %d:\n", i+1)
			fmt.Printf("  ID: %s\n", file.ID)
			fmt.Printf("  Name: %s\n", file.Name)
			fmt.Printf("  Title: %s\n", file.Title)
			fmt.Printf("  Mimetype: %s\n", file.Mimetype)
			fmt.Printf("  Size: %d bytes\n", file.Size)
			fmt.Printf("  URL: %s\n", file.URLPrivate)
			fmt.Printf("  URLPrivateDownload: %s\n", file.URLPrivateDownload)
			fmt.Printf("  Permalink: %s\n", file.Permalink)
			fmt.Printf("  IsExternal: %v\n", file.IsExternal)

			// サムネイル情報があれば表示
			if file.Thumb360 != "" {
				fmt.Printf("  Thumb360: %s\n", file.Thumb360)
			}
			if file.Thumb480 != "" {
				fmt.Printf("  Thumb480: %s\n", file.Thumb480)
			}
		}
	} else {
		fmt.Println("=== No files attached ===")
	}

	// 完全なJSONも出力
	fmt.Printf("\n=== Full Message JSON ===\n")
	msgJSON, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
	} else {
		fmt.Println(string(msgJSON))
	}

	// app_mention イベントのシミュレーション
	fmt.Printf("\n=== Simulated AppMentionEvent ===\n")
	appMentionEvent := map[string]interface{}{
		"type":      "app_mention",
		"user":      msg.User,
		"text":      msg.Text,
		"ts":        msg.Timestamp,
		"channel":   channelID,
		"event_ts":  msg.Timestamp,
		"thread_ts": msg.ThreadTimestamp,
		// ファイル情報は通常含まれない
	}

	eventJSON, err := json.MarshalIndent(appMentionEvent, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal event: %v", err)
	} else {
		fmt.Println(string(eventJSON))
	}
}
