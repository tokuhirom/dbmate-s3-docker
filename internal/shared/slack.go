package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// SendSlackNotification sends a notification to Slack webhook
func SendSlackNotification(ctx context.Context, webhookURL string, version string, result *Result) error {
	// Determine color and emoji
	color := "good"
	emoji := "✅"
	if result.Status != "success" {
		color = "danger"
		emoji = "❌"
	}

	// Truncate log to 1000 chars (same as shell script)
	logExcerpt := result.Log
	if len(logExcerpt) > 1000 {
		logExcerpt = logExcerpt[:1000]
	}

	payload := SlackPayload{
		Attachments: []SlackAttachment{
			{
				Color: color,
				Title: fmt.Sprintf("%s Migration %s", emoji, result.Status),
				Fields: []SlackField{
					{Title: "Version", Value: version, Short: true},
					{Title: "Status", Value: result.Status, Short: true},
				},
				Text: fmt.Sprintf("```\n%s\n```", logExcerpt),
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("Slack API returned status %d: %s", resp.StatusCode, string(body))
	}

	slog.Info("Slack notification sent successfully")
	return nil
}
