package shared

import (
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	migrationAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dbmate_migration_attempts_total",
			Help: "Total number of migration attempts",
		},
		[]string{"status"}, // success, failed
	)

	migrationDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "dbmate_migration_duration_seconds",
			Help:    "Duration of migration execution in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	lastMigrationTimestamp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "dbmate_last_migration_timestamp",
			Help: "Timestamp of the last migration (unix seconds)",
		},
	)

	currentVersion = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dbmate_current_version",
			Help: "Current migration version (labeled by version)",
		},
		[]string{"version"},
	)
)

// RecordMigrationAttempt records a migration attempt
func RecordMigrationAttempt(status string) {
	migrationAttempts.WithLabelValues(status).Inc()
}

// RecordMigrationDuration records the migration duration
func RecordMigrationDuration(seconds float64) {
	migrationDuration.Observe(seconds)
}

// RecordLastMigrationTimestamp records the last migration timestamp
func RecordLastMigrationTimestamp(timestamp float64) {
	lastMigrationTimestamp.Set(timestamp)
}

// RecordCurrentVersion records the current version
func RecordCurrentVersion(version string) {
	// Reset all version gauges
	currentVersion.Reset()
	// Set the current version to 1
	currentVersion.WithLabelValues(version).Set(1)
}

// StartMetricsServer starts the Prometheus metrics HTTP server
func StartMetricsServer(addr string) {
	http.Handle("/metrics", promhttp.Handler())
	slog.Info("Starting metrics server", "addr", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		slog.Error("Metrics server failed", "error", err)
	}
}
