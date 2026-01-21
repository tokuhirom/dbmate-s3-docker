package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/amacneil/dbmate/v2/pkg/dbmate"
	_ "github.com/amacneil/dbmate/v2/pkg/driver/postgres"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	// Version is set by the build process
	Version = "dev"
)

// CLI represents command line arguments
type CLI struct {
	DatabaseURL   string `help:"PostgreSQL connection string" env:"DATABASE_URL" required:""`
	S3Bucket      string `help:"S3 bucket name" env:"S3_BUCKET" required:""`
	S3PathPrefix  string `help:"S3 path prefix (e.g. 'migrations/')" env:"S3_PATH_PREFIX" required:""`
	S3EndpointURL string `help:"S3 endpoint URL (for S3-compatible services)" env:"S3_ENDPOINT_URL"`
	MetricsAddr   string `help:"Prometheus metrics endpoint address (e.g. ':9090')" env:"METRICS_ADDR"`

	Daemon        DaemonCmd        `cmd:"" help:"Run as daemon (default)" default:"1"`
	Once          OnceCmd          `cmd:"" help:"Run once and exit"`
	WaitAndNotify WaitAndNotifyCmd `cmd:"" help:"Wait for migration result and optionally notify Slack"`
	Version       VersionCmd       `cmd:"" help:"Show version information"`
}

// DaemonCmd runs as a daemon with periodic polling
type DaemonCmd struct {
	PollInterval time.Duration `help:"Polling interval for checking new versions" env:"POLL_INTERVAL" default:"30s"`
}

// OnceCmd runs once and exits
type OnceCmd struct {
}

// VersionCmd shows version information
type VersionCmd struct {
}

// WaitAndNotifyCmd waits for migration completion and optionally sends Slack notification
type WaitAndNotifyCmd struct {
	Version              string        `help:"Migration version to wait for (YYYYMMDDHHMMSS)" short:"v" required:""`
	SlackIncomingWebhook string        `help:"Slack incoming webhook URL (optional)" env:"SLACK_INCOMING_WEBHOOK"`
	Timeout              time.Duration `help:"Maximum wait time" default:"10m"`
	PollInterval         time.Duration `help:"Polling interval" default:"5s"`
}

// Result represents the migration execution result
type Result struct {
	Version           string `json:"version"`
	Status            string `json:"status"`
	Timestamp         string `json:"timestamp"`
	MigrationsApplied int    `json:"migrations_applied,omitempty"`
	Error             string `json:"error,omitempty"`
	Log               string `json:"log"`
}

// SlackPayload represents the Slack webhook payload
type SlackPayload struct {
	Attachments []SlackAttachment `json:"attachments"`
}

// SlackAttachment represents a Slack message attachment
type SlackAttachment struct {
	Color  string       `json:"color"`
	Title  string       `json:"title"`
	Fields []SlackField `json:"fields"`
	Text   string       `json:"text,omitempty"`
}

// SlackField represents a field in a Slack attachment
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("dbmate-s3-docker"),
		kong.Description("Database migration tool using dbmate with S3-based version management"),
		kong.UsageOnError(),
	)

	if err := ctx.Run(&cli); err != nil {
		slog.Error("Command failed", "error", err)
		os.Exit(1)
	}
}

func (cmd *DaemonCmd) Run(cli *CLI) error {
	ctx := context.Background()

	// Start metrics server if address is specified
	if cli.MetricsAddr != "" {
		go startMetricsServer(cli.MetricsAddr)
	}

	// Ensure prefix ends with /
	s3Prefix := cli.S3PathPrefix
	if !strings.HasSuffix(s3Prefix, "/") {
		s3Prefix += "/"
	}

	// Create S3 client
	s3Client, err := createS3Client(ctx, cli.S3EndpointURL)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	slog.Info("Starting database migration daemon", "poll_interval", cmd.PollInterval)

	// Create ticker for periodic polling
	ticker := time.NewTicker(cmd.PollInterval)
	defer ticker.Stop()

	// Run immediately on startup
	runMigrationCheck(ctx, s3Client, cli.S3Bucket, s3Prefix, cli.DatabaseURL)

	// Then run on ticker
	for range ticker.C {
		runMigrationCheck(ctx, s3Client, cli.S3Bucket, s3Prefix, cli.DatabaseURL)
	}

	return nil
}

func (cmd *OnceCmd) Run(cli *CLI) error {
	ctx := context.Background()

	// Start metrics server if address is specified
	if cli.MetricsAddr != "" {
		go startMetricsServer(cli.MetricsAddr)
	}

	// Ensure prefix ends with /
	s3Prefix := cli.S3PathPrefix
	if !strings.HasSuffix(s3Prefix, "/") {
		s3Prefix += "/"
	}

	// Create S3 client
	s3Client, err := createS3Client(ctx, cli.S3EndpointURL)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	slog.Info("Running migration check once")

	// Find unapplied version
	version, err := findUnappliedVersion(ctx, s3Client, cli.S3Bucket, s3Prefix)
	if err != nil {
		if err.Error() == "no unapplied versions found" {
			slog.Info("All versions are already applied")
			return nil
		}
		return fmt.Errorf("failed to find unapplied version: %w", err)
	}

	slog.Info("Found unapplied version", "version", version)

	// Execute migration with timing
	startTime := time.Now()
	result := executeMigration(ctx, s3Client, cli.S3Bucket, s3Prefix, version, cli.DatabaseURL)
	duration := time.Since(startTime).Seconds()

	// Record metrics
	recordMigrationDuration(duration)
	recordLastMigrationTimestamp(float64(time.Now().Unix()))
	if result.Status == "success" {
		recordMigrationAttempt("success")
		recordCurrentVersion(version)
	} else {
		recordMigrationAttempt("failed")
	}

	// Upload result (both success and failure)
	if err := uploadResult(ctx, s3Client, cli.S3Bucket, s3Prefix, version, result); err != nil {
		slog.Error("Failed to upload result", "error", err)
		return err
	}

	if result.Status != "success" {
		return fmt.Errorf("migration failed")
	}

	slog.Info("Migration completed successfully", "version", version)
	return nil
}

func (cmd *VersionCmd) Run(cli *CLI) error {
	fmt.Printf("dbmate-s3-docker version %s\n", Version)
	return nil
}

func runMigrationCheck(ctx context.Context, s3Client *s3.Client, bucket, prefix, databaseURL string) {
	slog.Info("Checking for unapplied migrations")

	// Find unapplied version
	version, err := findUnappliedVersion(ctx, s3Client, bucket, prefix)
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
	result := executeMigration(ctx, s3Client, bucket, prefix, version, databaseURL)
	duration := time.Since(startTime).Seconds()

	// Record metrics
	recordMigrationDuration(duration)
	recordLastMigrationTimestamp(float64(time.Now().Unix()))
	if result.Status == "success" {
		recordMigrationAttempt("success")
		recordCurrentVersion(version)
	} else {
		recordMigrationAttempt("failed")
	}

	// Upload result (both success and failure)
	if err := uploadResult(ctx, s3Client, bucket, prefix, version, result); err != nil {
		slog.Error("Failed to upload result", "error", err)
		return
	}

	if result.Status != "success" {
		slog.Error("Migration failed", "version", version)
		return
	}

	slog.Info("Migration completed successfully", "version", version)
}

func createS3Client(ctx context.Context, endpointURL string) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	if endpointURL != "" {
		client := s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpointURL)
			o.UsePathStyle = true
		})
		slog.Info("Using custom S3 endpoint", "endpoint", endpointURL)
		return client, nil
	}

	return s3.NewFromConfig(cfg), nil
}

func findUnappliedVersion(ctx context.Context, client *s3.Client, bucket, prefix string) (string, error) {
	slog.Info("Listing versions from S3", "bucket", bucket, "prefix", prefix)

	// List all objects with the prefix
	resp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list S3 objects: %w", err)
	}

	// Extract version directories
	var versions []string
	for _, cp := range resp.CommonPrefixes {
		if cp.Prefix == nil {
			continue
		}
		// Extract version from prefix (e.g., "migrations/20260121010000/" -> "20260121010000")
		versionPath := strings.TrimPrefix(*cp.Prefix, prefix)
		versionPath = strings.TrimSuffix(versionPath, "/")
		if versionPath != "" {
			versions = append(versions, versionPath)
		}
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no versions found")
	}

	// Sort versions numerically
	sort.Strings(versions)

	slog.Info("Found versions", "count", len(versions), "versions", versions)

	// Check the newest version (last in sorted list)
	newestVersion := versions[len(versions)-1]
	exists, err := checkResultExists(ctx, client, bucket, prefix, newestVersion)
	if err != nil {
		return "", fmt.Errorf("failed to check result.json for newest version %s: %w", newestVersion, err)
	}

	if !exists {
		slog.Info("Found unapplied newest version", "version", newestVersion)
		return newestVersion, nil
	}

	slog.Info("Newest version already applied (result.json exists)", "version", newestVersion)
	return "", fmt.Errorf("no unapplied versions found")
}

func checkResultExists(ctx context.Context, client *s3.Client, bucket, prefix, version string) (bool, error) {
	key := path.Join(prefix, version, "result.json")

	_, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchKey") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func executeMigration(ctx context.Context, client *s3.Client, bucket, prefix, version, databaseURL string) *Result {
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

	if err := downloadMigrations(ctx, client, bucket, migrationsPrefix, migrationsDir); err != nil {
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

func downloadMigrations(ctx context.Context, client *s3.Client, bucket, prefix, localDir string) error {
	// List all migration files
	resp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return err
	}

	// Download each file
	for _, obj := range resp.Contents {
		if obj.Key == nil {
			continue
		}

		key := *obj.Key
		fileName := path.Base(key)

		// Skip directory markers
		if fileName == "" || strings.HasSuffix(key, "/") {
			continue
		}

		// Download file
		result, err := client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return fmt.Errorf("failed to download %s: %w", key, err)
		}

		// Write to local file
		localPath := path.Join(localDir, fileName)
		file, err := os.Create(localPath)
		if err != nil {
			result.Body.Close()
			return fmt.Errorf("failed to create %s: %w", localPath, err)
		}

		_, err = io.Copy(file, result.Body)
		result.Body.Close()
		file.Close()

		if err != nil {
			return fmt.Errorf("failed to write %s: %w", localPath, err)
		}
	}

	return nil
}

func uploadResult(ctx context.Context, client *s3.Client, bucket, prefix, version string, result *Result) error {
	key := path.Join(prefix, version, "result.json")

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(jsonData),
	})

	if err != nil {
		return fmt.Errorf("failed to upload result: %w", err)
	}

	slog.Info("Result uploaded", "key", key)
	return nil
}

// downloadResult downloads and parses the result.json from S3
func downloadResult(ctx context.Context, client *s3.Client, bucket, prefix, version string) (*Result, error) {
	key := path.Join(prefix, version, "result.json")

	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get result from S3: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read result body: %w", err)
	}

	var result Result
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse result JSON: %w", err)
	}

	slog.Info("Downloaded and parsed result", "version", version, "status", result.Status)
	return &result, nil
}

// downloadResultWithRetry downloads result.json with exponential backoff retry
func downloadResultWithRetry(ctx context.Context, client *s3.Client, bucket, prefix, version string) (*Result, error) {
	backoff := time.Second
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		result, err := downloadResult(ctx, client, bucket, prefix, version)
		if err == nil {
			return result, nil
		}

		if attempt < maxRetries {
			slog.Warn("Failed to download result, retrying",
				"attempt", attempt,
				"max_retries", maxRetries,
				"backoff", backoff,
				"error", err)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	return nil, fmt.Errorf("failed to download result after %d attempts", maxRetries)
}

// waitForResult polls S3 for result.json until it appears or timeout occurs
func waitForResult(ctx context.Context, client *s3.Client, bucket, prefix, version string,
	pollInterval, timeout time.Duration) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	attempt := 0

	// Check immediately first (optimization)
	attempt++
	slog.Info("Checking for result", "version", version, "attempt", attempt)
	if exists, _ := checkResultExists(ctx, client, bucket, prefix, version); exists {
		slog.Info("Result found immediately", "version", version)
		return downloadResultWithRetry(ctx, client, bucket, prefix, version)
	}

	// Poll on interval
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for result after %v (checked %d times)", timeout, attempt)
		case <-ticker.C:
			attempt++
			slog.Info("Polling for result", "version", version, "attempt", attempt)

			exists, err := checkResultExists(ctx, client, bucket, prefix, version)
			if err != nil {
				slog.Warn("Error checking result existence", "error", err)
				continue // Retry on next interval
			}

			if exists {
				slog.Info("Result found", "version", version, "attempts", attempt)
				return downloadResultWithRetry(ctx, client, bucket, prefix, version)
			}
		}
	}
}

// sendSlackNotification sends a notification to Slack webhook
func sendSlackNotification(ctx context.Context, webhookURL string, version string, result *Result) error {
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

// Run executes the wait-and-notify command
func (cmd *WaitAndNotifyCmd) Run(cli *CLI) error {
	ctx := context.Background()

	// Ensure prefix ends with /
	s3Prefix := cli.S3PathPrefix
	if !strings.HasSuffix(s3Prefix, "/") {
		s3Prefix += "/"
	}

	// Create S3 client
	s3Client, err := createS3Client(ctx, cli.S3EndpointURL)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	hasSlackWebhook := cmd.SlackIncomingWebhook != ""

	slog.Info("Starting wait-and-notify",
		"version", cmd.Version,
		"slack_notification", hasSlackWebhook,
		"timeout", cmd.Timeout,
		"poll_interval", cmd.PollInterval)

	// Wait for result
	result, err := waitForResult(ctx, s3Client, cli.S3Bucket, s3Prefix,
		cmd.Version, cmd.PollInterval, cmd.Timeout)
	if err != nil {
		return err
	}

	// Send Slack notification if webhook URL provided
	if hasSlackWebhook {
		if err := sendSlackNotification(ctx, cmd.SlackIncomingWebhook, cmd.Version, result); err != nil {
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

	slog.Info("Migration completed successfully", "version", cmd.Version)
	return nil
}

