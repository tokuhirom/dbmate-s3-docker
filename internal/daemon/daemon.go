package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tokuhirom/dbmate-deployer/internal/shared"
)

// Cmd runs as a daemon with periodic polling
type Cmd struct {
	DatabaseURL  string        `help:"PostgreSQL connection string" env:"DATABASE_URL" required:""`
	S3Bucket     string        `help:"S3 bucket name" env:"S3_BUCKET" required:"" name:"s3-bucket"`
	S3PathPrefix string        `help:"S3 path prefix (e.g. 'migrations/')" env:"S3_PATH_PREFIX" required:"" name:"s3-path-prefix"`
	PollInterval time.Duration `help:"Polling interval for checking new versions" env:"POLL_INTERVAL" default:"30s"`
}

// Execute runs the daemon with periodic polling
func Execute(c *Cmd, s3EndpointURL, metricsAddr string) error {
	ctx := context.Background()

	// Start metrics server if address is specified
	if metricsAddr != "" {
		go shared.StartMetricsServer(metricsAddr)
	}

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

	slog.Info("Starting database migration daemon", "poll_interval", c.PollInterval)

	// Create ticker for periodic polling
	ticker := time.NewTicker(c.PollInterval)
	defer ticker.Stop()

	// Run immediately on startup
	runMigrationCheck(ctx, s3Client, c.S3Bucket, s3Prefix, c.DatabaseURL)

	// Then run on ticker
	for range ticker.C {
		runMigrationCheck(ctx, s3Client, c.S3Bucket, s3Prefix, c.DatabaseURL)
	}

	return nil
}

func runMigrationCheck(ctx context.Context, s3Client *s3.Client, bucket, prefix, databaseURL string) {
	slog.Info("Checking for unapplied migrations")

	// Find unapplied version
	version, err := shared.FindUnappliedVersion(ctx, s3Client, bucket, prefix)
	if err != nil {
		if err.Error() == "no unapplied versions found" {
			slog.Info("All versions are already applied")
			return
		}
		slog.Error("Failed to find unapplied version", "error", err)
		return
	}

	slog.Info("Found unapplied version", "version", version)

	// Execute migration with timing
	startTime := time.Now()
	result := shared.ExecuteMigration(ctx, s3Client, bucket, prefix, version, databaseURL)
	duration := time.Since(startTime).Seconds()

	// Record metrics
	shared.RecordMigrationDuration(duration)
	shared.RecordLastMigrationTimestamp(float64(time.Now().Unix()))
	if result.Status == "success" {
		shared.RecordMigrationAttempt("success")
		shared.RecordCurrentVersion(version)
	} else {
		shared.RecordMigrationAttempt("failed")
	}

	// Upload result (both success and failure)
	if err := shared.UploadResult(ctx, s3Client, bucket, prefix, version, result); err != nil {
		slog.Error("Failed to upload result", "error", err)
		return
	}

	if result.Status != "success" {
		slog.Error("Migration failed", "version", version)
		return
	}

	slog.Info("Migration completed successfully", "version", version)
}
