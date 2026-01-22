package shared

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMigrationFile(t *testing.T) {
	tests := []struct {
		name        string
		fileName    string
		content     string
		expectError bool
		errorMsg    string
	}{
		{
			name:     "valid migration with up and down markers",
			fileName: "20240101000000_create_users.sql",
			content: `-- migrate:up
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL
);

-- migrate:down
DROP TABLE users;
`,
			expectError: false,
		},
		{
			name:     "valid migration with only up marker",
			fileName: "20240101120000_add_index.sql",
			content: `-- migrate:up
CREATE INDEX idx_users_email ON users(email);
`,
			expectError: false,
		},
		{
			name:        "invalid: missing .sql extension",
			fileName:    "20240101000000_migration.txt",
			content:     "-- migrate:up\nCREATE TABLE test (id INT);",
			expectError: true,
			errorMsg:    "file must have .sql extension",
		},
		{
			name:        "invalid: filename too short",
			fileName:    "2024.sql",
			content:     "-- migrate:up\nCREATE TABLE test (id INT);",
			expectError: true,
			errorMsg:    "filename too short",
		},
		{
			name:        "invalid: non-numeric timestamp",
			fileName:    "20240ABC000000_migration.sql",
			content:     "-- migrate:up\nCREATE TABLE test (id INT);",
			expectError: true,
			errorMsg:    "must start with 14-digit timestamp",
		},
		{
			name:        "invalid: missing underscore after timestamp",
			fileName:    "20240101000000migration.sql",
			content:     "-- migrate:up\nCREATE TABLE test (id INT);",
			expectError: true,
			errorMsg:    "must have underscore after timestamp",
		},
		{
			name:        "invalid: missing migrate:up marker",
			fileName:    "20240101000000_no_marker.sql",
			content:     "CREATE TABLE test (id INT);",
			expectError: true,
			errorMsg:    "must contain '-- migrate:up' marker",
		},
		{
			name:     "valid: complex migration with multiple statements",
			fileName: "20240102000000_complex_migration.sql",
			content: `-- migrate:up
CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

INSERT INTO products (name) VALUES ('Product 1');
INSERT INTO products (name) VALUES ('Product 2');

CREATE INDEX idx_products_name ON products(name);

-- migrate:down
DROP TABLE products;
`,
			expectError: false,
		},
		{
			name:     "valid: migration with comments",
			fileName: "20240103000000_with_comments.sql",
			content: `-- This is a comment
-- Another comment

-- migrate:up
-- Create the main table
CREATE TABLE items (
    id SERIAL PRIMARY KEY,
    description TEXT
);

-- migrate:down
-- Remove the table
DROP TABLE items;
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file with the test content
			tempDir := t.TempDir()
			filePath := filepath.Join(tempDir, tt.fileName)

			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			require.NoError(t, err, "Failed to create test file")

			// Run validation
			err = ValidateMigrationFile(filePath)

			if tt.expectError {
				assert.Error(t, err, "Expected validation to fail")
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg, "Error message should contain expected text")
				}
			} else {
				assert.NoError(t, err, "Expected validation to pass")
			}
		})
	}
}

func TestValidateMigrationFile_FileNotFound(t *testing.T) {
	err := ValidateMigrationFile("/nonexistent/path/to/20240101000000_migration.sql")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestValidateMigrationFile_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "20240101000000_empty.sql")

	err := os.WriteFile(filePath, []byte(""), 0644)
	require.NoError(t, err)

	err = ValidateMigrationFile(filePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must contain '-- migrate:up' marker")
}
