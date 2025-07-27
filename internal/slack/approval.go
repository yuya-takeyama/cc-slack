package slack

import (
	"encoding/json"
	"fmt"
)

// BuildApprovalPayload builds the approval action payload
func BuildApprovalPayload(channelID, threadTS, message, requestID, action string) map[string]interface{} {
	return map[string]interface{}{
		"type": "block_actions",
		"actions": []map[string]interface{}{
			{
				"action_id": fmt.Sprintf("%s_%s", action, requestID),
				"type":      "button",
				"value":     action,
			},
		},
		"channel": map[string]string{
			"id": channelID,
		},
		"message": map[string]string{
			"ts":   threadTS,
			"text": message,
		},
	}
}

// GenerateDebugCurlCommands generates debug curl commands for approval actions
func GenerateDebugCurlCommands(approvePayload, denyPayload map[string]interface{}) string {
	approveJSON, _ := json.Marshal(approvePayload)
	denyJSON, _ := json.Marshal(denyPayload)

	return "*【デバッグ用curlコマンド】*\n" +
		"```bash\n" +
		"# 承認する場合:\n" +
		"curl -X POST http://localhost:8080/slack/interactive \\\n" +
		"  -H \"Content-Type: application/x-www-form-urlencoded\" \\\n" +
		fmt.Sprintf("  --data-urlencode 'payload=%s'\n\n", string(approveJSON)) +
		"# 拒否する場合:\n" +
		"curl -X POST http://localhost:8080/slack/interactive \\\n" +
		"  -H \"Content-Type: application/x-www-form-urlencoded\" \\\n" +
		fmt.Sprintf("  --data-urlencode 'payload=%s'\n", string(denyJSON)) +
		"```"
}
