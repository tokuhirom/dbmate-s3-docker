package testhelpers

import (
	"fmt"
	"os"
	"path/filepath"
)

// ValidMigration returns a valid migration SQL with up and down sections
func ValidMigration(tableName string) string {
	return fmt.Sprintf(`-- migrate:up
CREATE TABLE %s (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- migrate:down
DROP TABLE %s;
`, tableName, tableName)
}

// ValidMigrationWithData returns a valid migration that creates a table and inserts data
func ValidMigrationWithData(tableName string, rowCount int) string {
	sql := fmt.Sprintf(`-- migrate:up
CREATE TABLE %s (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

`, tableName)

	for i := 1; i <= rowCount; i++ {
		sql += fmt.Sprintf("INSERT INTO %s (name) VALUES ('row_%d');\n", tableName, i)
	}

	sql += fmt.Sprintf(`
-- migrate:down
DROP TABLE %s;
`, tableName)

	return sql
}

// InvalidMigrationMissingUp returns a migration missing the -- migrate:up marker
func InvalidMigrationMissingUp(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE %s (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

-- migrate:down
DROP TABLE %s;
`, tableName, tableName)
}

// InvalidMigrationMissingDown returns a migration missing the -- migrate:down marker
func InvalidMigrationMissingDown(tableName string) string {
	return fmt.Sprintf(`-- migrate:up
CREATE TABLE %s (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);
`, tableName)
}

// InvalidMigrationSyntaxError returns a migration with SQL syntax errors
func InvalidMigrationSyntaxError() string {
	return `-- migrate:up
CREATE TABLE invalid_syntax (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    CONSTRAINT INVALID SYNTAX HERE
);

-- migrate:down
DROP TABLE invalid_syntax;
`
}

// SuccessResult returns a JSON result for a successful migration
func SuccessResult(version, message string) string {
	return fmt.Sprintf(`{
  "status": "success",
  "version": "%s",
  "message": "%s",
  "timestamp": "2024-01-01T00:00:00Z"
}`, version, message)
}

// ErrorResult returns a JSON result for a failed migration
func ErrorResult(version, errorMsg string) string {
	return fmt.Sprintf(`{
  "status": "error",
  "version": "%s",
  "error": "%s",
  "timestamp": "2024-01-01T00:00:00Z"
}`, version, errorMsg)
}

// StandardMigrationVersions returns commonly used test migration versions
func StandardMigrationVersions() []string {
	return []string{
		"20240101000000",
		"20240101120000",
		"20240102000000",
		"20240103000000",
	}
}

// CreateMigrationSet creates a set of valid migrations for testing
func CreateMigrationSet(count int) map[string]string {
	migrations := make(map[string]string)
	versions := StandardMigrationVersions()

	for i := 0; i < count && i < len(versions); i++ {
		tableName := fmt.Sprintf("test_table_%d", i+1)
		migrations[versions[i]] = ValidMigration(tableName)
	}

	return migrations
}

// SlackWebhookPayload returns a sample Slack webhook payload
func SlackWebhookPayload(status, version, message string) string {
	var color string
	switch status {
	case "success":
		color = "#36a64f"
	case "error":
		color = "#ff0000"
	default:
		color = "#ffaa00"
	}

	return fmt.Sprintf(`{
  "attachments": [
    {
      "color": "%s",
      "title": "Migration %s",
      "text": "Version: %s\n%s",
      "footer": "dbmate-s3-docker",
      "ts": 1704067200
    }
  ]
}`, color, status, version, message)
}

// PostgresConnectionParams returns common PostgreSQL connection parameters for testing
type PostgresConnectionParams struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string
	SSLMode  string
}

// DefaultPostgresParams returns default test database parameters
func DefaultPostgresParams() PostgresConnectionParams {
	return PostgresConnectionParams{
		Host:     "localhost",
		Port:     "5432",
		Database: "testdb",
		Username: "testuser",
		Password: "testpass",
		SSLMode:  "disable",
	}
}

// BuildDatabaseURL constructs a PostgreSQL connection URL from parameters
func BuildDatabaseURL(params PostgresConnectionParams) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		params.Username,
		params.Password,
		params.Host,
		params.Port,
		params.Database,
		params.SSLMode,
	)
}

// WriteFile writes content to a file in the specified directory
func WriteFile(dir, filename, content string) error {
	filePath := filepath.Join(dir, filename)
	return os.WriteFile(filePath, []byte(content), 0644)
}
