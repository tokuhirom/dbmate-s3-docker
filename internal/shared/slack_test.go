package shared

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendSlackNotification_Success(t *testing.T) {
	// Create test server to receive webhook
	var receivedPayload SlackPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		err = json.Unmarshal(body, &receivedPayload)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	// Test successful migration
	result := &Result{
		Version:           "20240101000000",
		Status:            "success",
		Timestamp:         "2024-01-01T00:00:00Z",
		MigrationsApplied: 3,
		Log:               "Migration completed successfully",
	}

	err := SendSlackNotification(context.Background(), server.URL, "20240101000000", result)
	require.NoError(t, err)

	// Verify payload
	require.Len(t, receivedPayload.Attachments, 1)
	attachment := receivedPayload.Attachments[0]

	assert.Equal(t, "good", attachment.Color)
	assert.Contains(t, attachment.Title, "✅")
	assert.Contains(t, attachment.Title, "success")
	assert.Contains(t, attachment.Text, "Migration completed successfully")

	// Verify fields
	require.Len(t, attachment.Fields, 2)
	assert.Equal(t, "Version", attachment.Fields[0].Title)
	assert.Equal(t, "20240101000000", attachment.Fields[0].Value)
	assert.Equal(t, "Status", attachment.Fields[1].Title)
	assert.Equal(t, "success", attachment.Fields[1].Value)
}

func TestSendSlackNotification_Error(t *testing.T) {
	// Create test server to receive webhook
	var receivedPayload SlackPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		err = json.Unmarshal(body, &receivedPayload)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Test failed migration
	result := &Result{
		Version:   "20240101000000",
		Status:    "failed",
		Timestamp: "2024-01-01T00:00:00Z",
		Error:     "Database connection failed",
		Log:       "Error log content",
	}

	err := SendSlackNotification(context.Background(), server.URL, "20240101000000", result)
	require.NoError(t, err)

	// Verify payload for error case
	require.Len(t, receivedPayload.Attachments, 1)
	attachment := receivedPayload.Attachments[0]

	assert.Equal(t, "danger", attachment.Color)
	assert.Contains(t, attachment.Title, "❌")
	assert.Contains(t, attachment.Title, "failed")
}

func TestSendSlackNotification_LogTruncation(t *testing.T) {
	// Create test server
	var receivedPayload SlackPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		err = json.Unmarshal(body, &receivedPayload)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create long log (over 1000 characters)
	longLog := strings.Repeat("This is a long log line.\n", 50) // ~1250 characters

	result := &Result{
		Version:   "20240101000000",
		Status:    "success",
		Timestamp: "2024-01-01T00:00:00Z",
		Log:       longLog,
	}

	err := SendSlackNotification(context.Background(), server.URL, "20240101000000", result)
	require.NoError(t, err)

	// Verify log was truncated in the text field
	attachment := receivedPayload.Attachments[0]
	// Remove the code block markers (``` and newlines) to check the actual log length
	textContent := strings.TrimPrefix(attachment.Text, "```\n")
	textContent = strings.TrimSuffix(textContent, "\n```")

	// Should be truncated to 1000 characters
	assert.LessOrEqual(t, len(textContent), 1000)
}

func TestSendSlackNotification_ServerError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	result := &Result{
		Version:   "20240101000000",
		Status:    "success",
		Timestamp: "2024-01-01T00:00:00Z",
		Log:       "Test log",
	}

	err := SendSlackNotification(context.Background(), server.URL, "20240101000000", result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Slack API returned status 500")
	assert.Contains(t, err.Error(), "Internal Server Error")
}

func TestSendSlackNotification_InvalidURL(t *testing.T) {
	result := &Result{
		Version:   "20240101000000",
		Status:    "success",
		Timestamp: "2024-01-01T00:00:00Z",
		Log:       "Test log",
	}

	// Test with invalid URL that will cause network error
	err := SendSlackNotification(context.Background(), "http://invalid-host-that-does-not-exist-12345.com", "20240101000000", result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send Slack notification")
}

func TestSendSlackNotification_ContextCancellation(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This should not be reached due to context cancellation
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	result := &Result{
		Version:   "20240101000000",
		Status:    "success",
		Timestamp: "2024-01-01T00:00:00Z",
		Log:       "Test log",
	}

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := SendSlackNotification(ctx, server.URL, "20240101000000", result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send Slack notification")
}

func TestSlackPayloadFormat(t *testing.T) {
	// Test that the payload structure can be properly marshaled
	payload := SlackPayload{
		Attachments: []SlackAttachment{
			{
				Color: "good",
				Title: "Test Title",
				Fields: []SlackField{
					{Title: "Field1", Value: "Value1", Short: true},
					{Title: "Field2", Value: "Value2", Short: true},
				},
				Text: "Test text",
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	// Verify it's valid JSON
	var decoded SlackPayload
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, payload.Attachments[0].Color, decoded.Attachments[0].Color)
	assert.Equal(t, payload.Attachments[0].Title, decoded.Attachments[0].Title)
	assert.Len(t, decoded.Attachments[0].Fields, 2)
}
