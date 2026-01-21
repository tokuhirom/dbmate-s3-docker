package shared

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/amacneil/dbmate/v2/pkg/dbmate"
	_ "github.com/amacneil/dbmate/v2/pkg/driver/postgres"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ExecuteMigration executes database migration for a specific version
func ExecuteMigration(ctx context.Context, client *s3.Client, bucket, prefix, version, databaseURL string) *Result {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	var logBuffer bytes.Buffer

	result := &Result{
		Version:   version,
		Timestamp: timestamp,
	}

	log := func(msg string) {
		line := fmt.Sprintf("[%s] %s\n", time.Now().UTC().Format("2006-01-02 15:04:05 UTC"), msg)
		logBuffer.WriteString(line)
		slog.Info(msg)
	}

	log("=== Starting database migration ===")
	log(fmt.Sprintf("Version: %s", version))

	// Create temporary migrations directory
	migrationsDir, err := os.MkdirTemp("", "migrations-*")
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("Failed to create temp directory: %v", err)
		result.Log = logBuffer.String()
		return result
	}
	defer os.RemoveAll(migrationsDir)

	// Download migrations from S3
	migrationsPrefix := path.Join(prefix, version, "migrations") + "/"
	log(fmt.Sprintf("Downloading migrations from s3://%s/%s", bucket, migrationsPrefix))

	if err := DownloadMigrations(ctx, client, bucket, migrationsPrefix, migrationsDir); err != nil {
		log(fmt.Sprintf("✗ Failed to download migrations: %v", err))
		result.Status = "failed"
		result.Error = fmt.Sprintf("Failed to download migrations: %v", err)
		result.Log = logBuffer.String()
		return result
	}

	// Count migration files
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		log(fmt.Sprintf("✗ Failed to read migrations directory: %v", err))
		result.Status = "failed"
		result.Error = fmt.Sprintf("Failed to read migrations directory: %v", err)
		result.Log = logBuffer.String()
		return result
	}

	migrationCount := len(files)
	log(fmt.Sprintf("Downloaded %d migration files", migrationCount))

	// Run dbmate using library
	log("Running dbmate up...")

	u, err := url.Parse(databaseURL)
	if err != nil {
		log(fmt.Sprintf("✗ Failed to parse DATABASE_URL: %v", err))
		result.Status = "failed"
		result.Error = fmt.Sprintf("Invalid DATABASE_URL: %v", err)
		result.Log = logBuffer.String()
		return result
	}

	db := dbmate.New(u)
	db.MigrationsDir = []string{migrationsDir}
	db.AutoDumpSchema = false

	if err := db.CreateAndMigrate(); err != nil {
		log(fmt.Sprintf("✗ Migration failed: %v", err))
		result.Status = "failed"
		result.Error = fmt.Sprintf("dbmate failed: %v", err)
		result.Log = logBuffer.String()
		return result
	}

	log("✓ Migration completed successfully")

	result.Status = "success"
	result.MigrationsApplied = migrationCount
	result.Log = logBuffer.String()

	return result
}

// ValidateMigrationFile validates a migration file's format and content
func ValidateMigrationFile(filePath string) error {
	// Check filename format: YYYYMMDDHHMMSS_description.sql
	fileName := path.Base(filePath)

	// Must end with .sql
	if !strings.HasSuffix(fileName, ".sql") {
		return fmt.Errorf("file must have .sql extension: %s", fileName)
	}

	// Check if filename starts with timestamp (14 digits)
	if len(fileName) < 15 { // YYYYMMDDHHMMSS + _ + at least 1 char + .sql
		return fmt.Errorf("filename too short, expected format: YYYYMMDDHHMMSS_description.sql: %s", fileName)
	}

	// Check first 14 characters are digits
	timestamp := fileName[:14]
	for _, c := range timestamp {
		if c < '0' || c > '9' {
			return fmt.Errorf("filename must start with 14-digit timestamp (YYYYMMDDHHMMSS): %s", fileName)
		}
	}

	// Check underscore after timestamp
	if fileName[14] != '_' {
		return fmt.Errorf("filename must have underscore after timestamp: %s", fileName)
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	contentStr := string(content)

	// Check for required "-- migrate:up" marker
	if !strings.Contains(contentStr, "-- migrate:up") {
		return fmt.Errorf("migration file must contain '-- migrate:up' marker: %s", fileName)
	}

	// Check for recommended "-- migrate:down" marker (warning only)
	if !strings.Contains(contentStr, "-- migrate:down") {
		slog.Warn("Migration file missing '-- migrate:down' marker (not required but recommended)", "file", fileName)
	}

	return nil
}
