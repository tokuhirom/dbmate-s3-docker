package push

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/tokuhirom/dbmate-s3-docker/internal/shared"
)

// Cmd uploads migration files to S3
type Cmd struct {
	MigrationsDir string `help:"Local directory containing migration files" required:"" type:"path" name:"migrations-dir" short:"m"`
	S3Bucket      string `help:"S3 bucket name" env:"S3_BUCKET" required:"" name:"s3-bucket"`
	S3PathPrefix  string `help:"S3 path prefix (e.g. 'migrations/')" env:"S3_PATH_PREFIX" required:"" name:"s3-path-prefix"`
	Version       string `help:"Version timestamp (YYYYMMDDHHMMSS)" required:"" name:"version" short:"v"`
	DryRun        bool   `help:"Show what would be uploaded without uploading" name:"dry-run"`
	Validate      bool   `help:"Validate migration files before upload" default:"true" name:"validate"`
}

// Execute runs the push command
func Execute(c *Cmd, s3EndpointURL, metricsAddr string) error {
	ctx := context.Background()

	// Validate version format (14 digits)
	if len(c.Version) != 14 {
		return fmt.Errorf("version must be 14 digits (YYYYMMDDHHMMSS): %s", c.Version)
	}
	for _, ch := range c.Version {
		if ch < '0' || ch > '9' {
			return fmt.Errorf("version must contain only digits: %s", c.Version)
		}
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

	// Check if version already exists
	exists, err := shared.CheckResultExists(ctx, s3Client, c.S3Bucket, s3Prefix, c.Version)
	if err != nil {
		return fmt.Errorf("failed to check if version exists: %w", err)
	}
	if exists {
		return fmt.Errorf("version %s already exists", c.Version)
	}

	// Read and filter migration files
	entries, err := os.ReadDir(c.MigrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var sqlFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".sql") {
			sqlFiles = append(sqlFiles, entry.Name())
		}
	}

	if len(sqlFiles) == 0 {
		return fmt.Errorf("no .sql files found in directory: %s", c.MigrationsDir)
	}

	slog.Info("Found migration files", "count", len(sqlFiles))

	// Validate migration files if requested
	if c.Validate {
		slog.Info("Validating migration files")
		for _, fileName := range sqlFiles {
			filePath := path.Join(c.MigrationsDir, fileName)
			if err := shared.ValidateMigrationFile(filePath); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
		}
		slog.Info("All migration files validated successfully")
	}

	// Dry-run mode
	if c.DryRun {
		fmt.Println("Dry-run mode: would upload the following files:")
		for _, fileName := range sqlFiles {
			s3Key := path.Join(s3Prefix, c.Version, "migrations", fileName)
			fmt.Printf("  %s -> s3://%s/%s\n", fileName, c.S3Bucket, s3Key)
		}
		fmt.Printf("\nVersion: %s\n", c.Version)
		return nil
	}

	// Upload migrations
	slog.Info("Uploading migrations to S3", "bucket", c.S3Bucket, "prefix", s3Prefix, "version", c.Version)
	if err := shared.UploadMigrations(ctx, s3Client, c.S3Bucket, s3Prefix, c.Version, c.MigrationsDir); err != nil {
		return fmt.Errorf("failed to upload migrations: %w", err)
	}

	slog.Info("Successfully uploaded migrations", "version", c.Version, "count", len(sqlFiles))
	fmt.Printf("Version: %s\n", c.Version)

	return nil
}
