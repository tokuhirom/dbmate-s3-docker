package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3API defines the interface for S3 operations used in this application
// This interface enables mocking for unit tests
type S3API interface {
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// CreateS3Client creates an S3 client with optional custom endpoint
func CreateS3Client(ctx context.Context, endpointURL string) (*s3.Client, error) {
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

// FindUnappliedVersion finds the newest unapplied migration version
func FindUnappliedVersion(ctx context.Context, client S3API, bucket, prefix string) (string, error) {
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
	exists, err := CheckResultExists(ctx, client, bucket, prefix, newestVersion)
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

// CheckResultExists checks if result.json exists for a version
func CheckResultExists(ctx context.Context, client S3API, bucket, prefix, version string) (bool, error) {
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

// DownloadMigrations downloads migration files from S3 to a local directory
func DownloadMigrations(ctx context.Context, client S3API, bucket, prefix, localDir string) error {
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

// UploadMigrations uploads migration files from a local directory to S3
func UploadMigrations(ctx context.Context, client S3API, bucket, prefix, version, localDir string) error {
	// Read directory entries
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// Filter .sql files
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
		return fmt.Errorf("no .sql files found in directory: %s", localDir)
	}

	slog.Info("Uploading migration files", "count", len(sqlFiles))

	// Upload each file
	for _, fileName := range sqlFiles {
		localPath := path.Join(localDir, fileName)

		// Read file content
		content, err := os.ReadFile(localPath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", fileName, err)
		}

		// Construct S3 key
		s3Key := path.Join(prefix, version, "migrations", fileName)

		// Upload to S3
		_, err = client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(s3Key),
			Body:   bytes.NewReader(content),
		})
		if err != nil {
			return fmt.Errorf("failed to upload %s: %w", fileName, err)
		}

		slog.Info("Uploaded file", "file", fileName, "s3_key", s3Key)
	}

	return nil
}

// UploadResult uploads the migration result as JSON to S3
func UploadResult(ctx context.Context, client S3API, bucket, prefix, version string, result *Result) error {
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
func downloadResult(ctx context.Context, client S3API, bucket, prefix, version string) (*Result, error) {
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
func downloadResultWithRetry(ctx context.Context, client S3API, bucket, prefix, version string) (*Result, error) {
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

// WaitForResult polls S3 for result.json until it appears or timeout occurs
func WaitForResult(ctx context.Context, client S3API, bucket, prefix, version string,
	pollInterval, timeout time.Duration) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	attempt := 0

	// Check immediately first (optimization)
	attempt++
	slog.Info("Checking for result", "version", version, "attempt", attempt)
	if exists, _ := CheckResultExists(ctx, client, bucket, prefix, version); exists {
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

			exists, err := CheckResultExists(ctx, client, bucket, prefix, version)
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
