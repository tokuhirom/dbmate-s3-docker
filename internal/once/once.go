package once

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/tokuhirom/dbmate-deployer/internal/shared"
)

// Cmd runs once and exits
type Cmd struct {
	DatabaseURL  string `help:"PostgreSQL connection string" env:"DATABASE_URL" required:""`
	S3Bucket     string `help:"S3 bucket name" env:"S3_BUCKET" required:"" name:"s3-bucket"`
	S3PathPrefix string `help:"S3 path prefix (e.g. 'migrations/')" env:"S3_PATH_PREFIX" required:"" name:"s3-path-prefix"`
}

// Execute runs the migration check once and exits
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

	slog.Info("Running migration check once")

	// Find unapplied version
	version, err := shared.FindUnappliedVersion(ctx, s3Client, c.S3Bucket, s3Prefix)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "no unapplied versions found" {
			slog.Info("All versions are already applied")
			return nil
		}
		if errMsg == "no versions found" {
			slog.Info("No migration versions found in S3")
			return nil
		}
		return fmt.Errorf("failed to find unapplied version: %w", err)
	}

	slog.Info("Found unapplied version", "version", version)

	// Execute migration with timing
	startTime := time.Now()
	result := shared.ExecuteMigration(ctx, s3Client, c.S3Bucket, s3Prefix, version, c.DatabaseURL)
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
	if err := shared.UploadResult(ctx, s3Client, c.S3Bucket, s3Prefix, version, result); err != nil {
		slog.Error("Failed to upload result", "error", err)
		return err
	}

	if result.Status != "success" {
		return fmt.Errorf("migration failed")
	}

	slog.Info("Migration completed successfully", "version", version)
	return nil
}
