package wait

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/tokuhirom/dbmate-s3-docker/internal/shared"
)

// Cmd waits for migration completion and optionally sends Slack notification
type Cmd struct {
	S3Bucket             string        `help:"S3 bucket name" env:"S3_BUCKET" required:"" name:"s3-bucket"`
	S3PathPrefix         string        `help:"S3 path prefix (e.g. 'migrations/')" env:"S3_PATH_PREFIX" required:"" name:"s3-path-prefix"`
	MigrationVersion     string        `help:"Migration version to wait for (YYYYMMDDHHMMSS)" short:"v" required:""`
	SlackIncomingWebhook string        `help:"Slack incoming webhook URL (optional)" env:"SLACK_INCOMING_WEBHOOK"`
	Timeout              time.Duration `help:"Maximum wait time" default:"10m"`
	PollInterval         time.Duration `help:"Polling interval" default:"5s"`
}

// Execute waits for migration completion and optionally notifies Slack
func Execute(c *Cmd, s3EndpointURL, metricsAddr string) error {
	ctx := context.Background()

	// Ensure prefix ends with /
	s3Prefix := c.S3PathPrefix
	if !strings.HasSuffix(s3Prefix, "/") {
		s3Prefix += "/"
	}

	// Create S3 client
	s3Client, err := shared.CreateS3Client(ctx, s3EndpointURL)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	hasSlackWebhook := c.SlackIncomingWebhook != ""

	slog.Info("Starting wait-and-notify",
		"version", c.MigrationVersion,
		"slack_notification", hasSlackWebhook,
		"timeout", c.Timeout,
		"poll_interval", c.PollInterval)

	// Wait for result
	result, err := shared.WaitForResult(ctx, s3Client, c.S3Bucket, s3Prefix,
		c.MigrationVersion, c.PollInterval, c.Timeout)
	if err != nil {
		return err
	}

	// Send Slack notification if webhook URL provided
	if hasSlackWebhook {
		if err := shared.SendSlackNotification(ctx, c.SlackIncomingWebhook, c.MigrationVersion, result); err != nil {
			slog.Warn("Failed to send Slack notification", "error", err)
			// Continue - notification failure shouldn't fail the command
		}
	} else {
		slog.Info("Slack webhook not configured, skipping notification")
	}

	// Exit with appropriate status
	if result.Status != "success" {
		return fmt.Errorf("migration failed: %s", result.Error)
	}

	slog.Info("Migration completed successfully", "version", c.MigrationVersion)
	return nil
}
