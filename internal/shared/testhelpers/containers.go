package testhelpers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/lib/pq"
)

// TestEnvironment holds all test infrastructure for integration tests
type TestEnvironment struct {
	PostgresContainer testcontainers.Container
	LocalStackContainer testcontainers.Container
	DatabaseURL       string
	S3Client          *s3.Client
	S3Bucket          string
	S3EndpointURL     string
	DB                *sql.DB
	t                 *testing.T
}

// SetupPostgresContainer starts a PostgreSQL container and returns the connection string
func SetupPostgresContainer(ctx context.Context, t *testing.T) (testcontainers.Container, string) {
	t.Helper()

	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err, "Failed to start PostgreSQL container")

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get PostgreSQL connection string")

	return postgresContainer, connStr
}

// SetupLocalStackContainer starts a LocalStack container for S3 testing
func SetupLocalStackContainer(ctx context.Context, t *testing.T) (testcontainers.Container, string, *s3.Client) {
	t.Helper()

	localstackContainer, err := localstack.RunContainer(ctx,
		testcontainers.WithImage("localstack/localstack:3.0"),
		testcontainers.WithEnv(map[string]string{
			"SERVICES": "s3",
		}),
	)
	require.NoError(t, err, "Failed to start LocalStack container")

	// Get the mapped endpoint
	mappedEndpoint, err := localstackContainer.PortEndpoint(ctx, "4566", "http")
	require.NoError(t, err, "Failed to get LocalStack endpoint")

	// Create S3 client configured for LocalStack
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test",
			"test",
			"",
		)),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               mappedEndpoint,
					HostnameImmutable: true,
					SigningRegion:     "us-east-1",
				}, nil
			},
		)),
	)
	require.NoError(t, err, "Failed to create AWS config")

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return localstackContainer, mappedEndpoint, s3Client
}

// SetupTestEnvironment creates a complete test environment with PostgreSQL and LocalStack
func SetupTestEnvironment(ctx context.Context, t *testing.T) *TestEnvironment {
	t.Helper()

	// Start PostgreSQL
	postgresContainer, dbURL := SetupPostgresContainer(ctx, t)

	// Start LocalStack
	localstackContainer, endpoint, s3Client := SetupLocalStackContainer(ctx, t)

	// Open database connection
	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err, "Failed to open database connection")

	// Verify database connection
	err = db.Ping()
	require.NoError(t, err, "Failed to ping database")

	// Create test bucket
	bucketName := "test-migrations"
	_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	require.NoError(t, err, "Failed to create test S3 bucket")

	env := &TestEnvironment{
		PostgresContainer:   postgresContainer,
		LocalStackContainer: localstackContainer,
		DatabaseURL:         dbURL,
		S3Client:            s3Client,
		S3Bucket:            bucketName,
		S3EndpointURL:       endpoint,
		DB:                  db,
		t:                   t,
	}

	// Register cleanup
	t.Cleanup(func() {
		env.Cleanup(ctx)
	})

	return env
}

// Cleanup terminates all containers and closes connections
func (e *TestEnvironment) Cleanup(ctx context.Context) {
	if e.DB != nil {
		e.DB.Close()
	}
	if e.PostgresContainer != nil {
		e.PostgresContainer.Terminate(ctx)
	}
	if e.LocalStackContainer != nil {
		e.LocalStackContainer.Terminate(ctx)
	}
}

// Reset clears all test data for reuse between tests
func (e *TestEnvironment) Reset(ctx context.Context) {
	e.t.Helper()

	// Clear all S3 objects
	e.ClearS3Bucket(ctx)

	// Drop all tables from database
	e.ClearDatabase(ctx)
}

// ClearS3Bucket removes all objects from the test bucket
func (e *TestEnvironment) ClearS3Bucket(ctx context.Context) {
	e.t.Helper()

	// List all objects
	listOutput, err := e.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(e.S3Bucket),
	})
	require.NoError(e.t, err, "Failed to list S3 objects")

	// Delete each object
	for _, obj := range listOutput.Contents {
		_, err := e.S3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(e.S3Bucket),
			Key:    obj.Key,
		})
		require.NoError(e.t, err, "Failed to delete S3 object")
	}
}

// ClearDatabase drops all tables from the test database
func (e *TestEnvironment) ClearDatabase(ctx context.Context) {
	e.t.Helper()

	// Drop schema_migrations table if it exists
	_, err := e.DB.ExecContext(ctx, "DROP TABLE IF EXISTS schema_migrations CASCADE")
	require.NoError(e.t, err, "Failed to drop schema_migrations table")

	// Get all user tables
	rows, err := e.DB.QueryContext(ctx, `
		SELECT tablename FROM pg_tables
		WHERE schemaname = 'public'
	`)
	require.NoError(e.t, err, "Failed to query tables")
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		err := rows.Scan(&tableName)
		require.NoError(e.t, err, "Failed to scan table name")
		tables = append(tables, tableName)
	}

	// Drop each table
	for _, table := range tables {
		_, err := e.DB.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table))
		require.NoError(e.t, err, "Failed to drop table %s", table)
	}
}

// UploadMigration uploads a single migration file to S3
func (e *TestEnvironment) UploadMigration(ctx context.Context, version, sqlContent string) {
	e.t.Helper()

	key := fmt.Sprintf("migrations/%s.sql", version)
	_, err := e.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(e.S3Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(sqlContent)),
	})
	require.NoError(e.t, err, "Failed to upload migration to S3")
}

// UploadMigrationFile uploads a migration file from filesystem to S3
func (e *TestEnvironment) UploadMigrationFile(ctx context.Context, version, filePath string) {
	e.t.Helper()

	content, err := os.ReadFile(filePath)
	require.NoError(e.t, err, "Failed to read migration file")

	e.UploadMigration(ctx, version, string(content))
}

// UploadMigrationsFromDir uploads all migration files from a directory to S3
func (e *TestEnvironment) UploadMigrationsFromDir(ctx context.Context, dirPath string) {
	e.t.Helper()

	files, err := os.ReadDir(dirPath)
	require.NoError(e.t, err, "Failed to read migrations directory")

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".sql" {
			continue
		}

		version := file.Name()[:len(file.Name())-4] // Remove .sql extension
		fullPath := filepath.Join(dirPath, file.Name())
		e.UploadMigrationFile(ctx, version, fullPath)
	}
}

// UploadResult uploads a result.json file to S3
func (e *TestEnvironment) UploadResult(ctx context.Context, version, resultJSON string) {
	e.t.Helper()

	key := fmt.Sprintf("results/%s/result.json", version)
	_, err := e.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(e.S3Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(resultJSON)),
	})
	require.NoError(e.t, err, "Failed to upload result to S3")
}

// GetResult retrieves and parses a result.json from S3
func (e *TestEnvironment) GetResult(ctx context.Context, version string) map[string]interface{} {
	e.t.Helper()

	key := fmt.Sprintf("results/%s/result.json", version)
	output, err := e.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(e.S3Bucket),
		Key:    aws.String(key),
	})
	require.NoError(e.t, err, "Failed to get result from S3")
	defer output.Body.Close()

	var result map[string]interface{}
	err = json.NewDecoder(output.Body).Decode(&result)
	require.NoError(e.t, err, "Failed to decode result JSON")

	return result
}

// ResultExists checks if a result file exists in S3
func (e *TestEnvironment) ResultExists(ctx context.Context, version string) bool {
	e.t.Helper()

	key := fmt.Sprintf("results/%s/result.json", version)
	_, err := e.S3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(e.S3Bucket),
		Key:    aws.String(key),
	})
	return err == nil
}

// AssertTableExists verifies that a table exists in the database
func (e *TestEnvironment) AssertTableExists(t *testing.T, tableName string) {
	t.Helper()

	var exists bool
	err := e.DB.QueryRow(`
		SELECT EXISTS (
			SELECT FROM pg_tables
			WHERE schemaname = 'public'
			AND tablename = $1
		)
	`, tableName).Scan(&exists)
	require.NoError(t, err, "Failed to check if table exists")
	require.True(t, exists, "Table %s should exist", tableName)
}

// AssertTableNotExists verifies that a table does not exist in the database
func (e *TestEnvironment) AssertTableNotExists(t *testing.T, tableName string) {
	t.Helper()

	var exists bool
	err := e.DB.QueryRow(`
		SELECT EXISTS (
			SELECT FROM pg_tables
			WHERE schemaname = 'public'
			AND tablename = $1
		)
	`, tableName).Scan(&exists)
	require.NoError(t, err, "Failed to check if table exists")
	require.False(t, exists, "Table %s should not exist", tableName)
}

// GetAppliedMigrations returns the list of applied migrations from schema_migrations
func (e *TestEnvironment) GetAppliedMigrations(ctx context.Context) []string {
	e.t.Helper()

	rows, err := e.DB.QueryContext(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		// Table might not exist yet
		return []string{}
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var version string
		err := rows.Scan(&version)
		require.NoError(e.t, err, "Failed to scan migration version")
		versions = append(versions, version)
	}

	return versions
}
