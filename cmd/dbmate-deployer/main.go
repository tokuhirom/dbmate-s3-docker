package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/tokuhirom/dbmate-deployer/internal/daemon"
	"github.com/tokuhirom/dbmate-deployer/internal/once"
	"github.com/tokuhirom/dbmate-deployer/internal/push"
	"github.com/tokuhirom/dbmate-deployer/internal/version"
	"github.com/tokuhirom/dbmate-deployer/internal/wait"
)

var (
	// Version is set by the build process
	Version = "dev"
)

// CLI represents command line arguments
type CLI struct {
	S3EndpointURL string `help:"S3 endpoint URL (for S3-compatible services)" env:"S3_ENDPOINT_URL" name:"s3-endpoint-url"`
	MetricsAddr   string `help:"Prometheus metrics endpoint address (e.g. ':9090')" env:"METRICS_ADDR"`

	Daemon        DaemonCmd        `cmd:"" help:"Run as daemon (default)" default:"1"`
	Once          OnceCmd          `cmd:"" help:"Run once and exit"`
	Push          PushCmd          `cmd:"" help:"Upload migrations to S3"`
	WaitAndNotify WaitAndNotifyCmd `cmd:"" help:"Wait for migration result and optionally notify Slack"`
	Version       VersionCmd       `cmd:"" help:"Show version information"`
}

// DaemonCmd runs as a daemon with periodic polling
type DaemonCmd struct {
	DatabaseURL  string        `help:"PostgreSQL connection string" env:"DATABASE_URL" required:""`
	S3Bucket     string        `help:"S3 bucket name" env:"S3_BUCKET" required:"" name:"s3-bucket"`
	S3PathPrefix string        `help:"S3 path prefix (e.g. 'migrations/')" env:"S3_PATH_PREFIX" required:"" name:"s3-path-prefix"`
	PollInterval time.Duration `help:"Polling interval for checking new versions" env:"POLL_INTERVAL" default:"30s"`
}

// OnceCmd runs once and exits
type OnceCmd struct {
	DatabaseURL  string `help:"PostgreSQL connection string" env:"DATABASE_URL" required:""`
	S3Bucket     string `help:"S3 bucket name" env:"S3_BUCKET" required:"" name:"s3-bucket"`
	S3PathPrefix string `help:"S3 path prefix (e.g. 'migrations/')" env:"S3_PATH_PREFIX" required:"" name:"s3-path-prefix"`
}

// PushCmd uploads migration files to S3
type PushCmd struct {
	MigrationsDir string `help:"Local directory containing migration files" required:"" type:"path" name:"migrations-dir" short:"m"`
	S3Bucket      string `help:"S3 bucket name" env:"S3_BUCKET" required:"" name:"s3-bucket"`
	S3PathPrefix  string `help:"S3 path prefix (e.g. 'migrations/')" env:"S3_PATH_PREFIX" required:"" name:"s3-path-prefix"`
	Version       string `help:"Version timestamp (YYYYMMDDHHMMSS)" required:"" name:"version" short:"v"`
	DryRun        bool   `help:"Show what would be uploaded without uploading" name:"dry-run"`
	Validate      bool   `help:"Validate migration files before upload" default:"true" name:"validate"`
}

// WaitAndNotifyCmd waits for migration completion and optionally sends Slack notification
type WaitAndNotifyCmd struct {
	S3Bucket             string        `help:"S3 bucket name" env:"S3_BUCKET" required:"" name:"s3-bucket"`
	S3PathPrefix         string        `help:"S3 path prefix (e.g. 'migrations/')" env:"S3_PATH_PREFIX" required:"" name:"s3-path-prefix"`
	MigrationVersion     string        `help:"Migration version to wait for (YYYYMMDDHHMMSS)" short:"v" required:""`
	SlackIncomingWebhook string        `help:"Slack incoming webhook URL (optional)" env:"SLACK_INCOMING_WEBHOOK"`
	Timeout              time.Duration `help:"Maximum wait time" default:"10m"`
	PollInterval         time.Duration `help:"Polling interval" default:"5s"`
}

// VersionCmd shows version information
type VersionCmd struct {
}

// Run() forwarders for each command (required by kong)
func (c *DaemonCmd) Run(cli *CLI) error {
	cmd := &daemon.Cmd{
		DatabaseURL:  c.DatabaseURL,
		S3Bucket:     c.S3Bucket,
		S3PathPrefix: c.S3PathPrefix,
		PollInterval: c.PollInterval,
	}
	return daemon.Execute(cmd, cli.S3EndpointURL, cli.MetricsAddr)
}

func (c *OnceCmd) Run(cli *CLI) error {
	cmd := &once.Cmd{
		DatabaseURL:  c.DatabaseURL,
		S3Bucket:     c.S3Bucket,
		S3PathPrefix: c.S3PathPrefix,
	}
	return once.Execute(cmd, cli.S3EndpointURL, cli.MetricsAddr)
}

func (c *PushCmd) Run(cli *CLI) error {
	cmd := &push.Cmd{
		MigrationsDir: c.MigrationsDir,
		S3Bucket:      c.S3Bucket,
		S3PathPrefix:  c.S3PathPrefix,
		Version:       c.Version,
		DryRun:        c.DryRun,
		Validate:      c.Validate,
	}
	return push.Execute(cmd, cli.S3EndpointURL, cli.MetricsAddr)
}

func (c *WaitAndNotifyCmd) Run(cli *CLI) error {
	cmd := &wait.Cmd{
		S3Bucket:             c.S3Bucket,
		S3PathPrefix:         c.S3PathPrefix,
		MigrationVersion:     c.MigrationVersion,
		SlackIncomingWebhook: c.SlackIncomingWebhook,
		Timeout:              c.Timeout,
		PollInterval:         c.PollInterval,
	}
	return wait.Execute(cmd, cli.S3EndpointURL, cli.MetricsAddr)
}

func (c *VersionCmd) Run(cli *CLI) error {
	cmd := &version.Cmd{}
	return version.Execute(cmd, Version)
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("dbmate-deployer"),
		kong.Description("Database migration deployment tool using dbmate with S3-based version management"),
		kong.UsageOnError(),
	)

	if err := ctx.Run(&cli); err != nil {
		slog.Error("Command failed", "error", err)
		os.Exit(1)
	}
}
