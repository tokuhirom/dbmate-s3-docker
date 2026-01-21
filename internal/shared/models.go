package shared

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
