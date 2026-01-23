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

// PushInfo represents metadata about when and where migrations were pushed from
type PushInfo struct {
	PushedAt string      `json:"pushed_at"`
	Source   PushSource  `json:"source"`
}

// PushSource represents the source of the push operation
type PushSource struct {
	Type       string `json:"type"`                  // "github_actions" or "local"
	Repository string `json:"repository,omitempty"` // GitHub repository (owner/repo)
	Workflow   string `json:"workflow,omitempty"`   // GitHub Actions workflow name
	RunID      string `json:"run_id,omitempty"`     // GitHub Actions run ID
	RunURL     string `json:"run_url,omitempty"`    // URL to the GitHub Actions run
	Actor      string `json:"actor,omitempty"`      // User or app that triggered the workflow
	SHA        string `json:"sha,omitempty"`        // Git commit SHA
	Ref        string `json:"ref,omitempty"`        // Git ref (branch or tag)
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
