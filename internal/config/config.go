// Package config loads and validates the application's environment-variable
// configuration. Load performs no I/O of its own — it reads only through the
// injected getenv function, so callers control exactly what it sees.
package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config holds every environment-variable setting the application needs.
type Config struct {
	TZ string

	MediaMoviesPath string
	MediaTVPath     string

	RadarrURL    string
	RadarrAPIKey string

	SonarrURL    string
	SonarrAPIKey string

	InfluxURL    string
	InfluxToken  string
	InfluxOrg    string
	InfluxBucket string

	SnapshotPath      string
	SnapshotRetention int

	MaxMediaBytesDecreasePercent float64
	MaxMovieDecreasePercent      float64
	MaxEpisodeDecreasePercent    float64
	MaxFilesRemoved              int
	MaxUnmatchedPercent          float64

	RcloneRemote string
}

// OffsiteEnabled reports whether off-site sync is configured. An empty
// RCLONE_REMOTE disables the step rather than being an error.
func (c Config) OffsiteEnabled() bool {
	return c.RcloneRemote != ""
}

// String renders the configuration with credentials redacted, suitable for
// logging or the `config` CLI subcommand.
func (c Config) String() string {
	redact := func(s string) string {
		if s == "" {
			return ""
		}
		return "REDACTED"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "TZ=%s\n", c.TZ)
	fmt.Fprintf(&b, "MEDIA_MOVIES_PATH=%s\n", c.MediaMoviesPath)
	fmt.Fprintf(&b, "MEDIA_TV_PATH=%s\n", c.MediaTVPath)
	fmt.Fprintf(&b, "RADARR_URL=%s\n", c.RadarrURL)
	fmt.Fprintf(&b, "RADARR_API_KEY=%s\n", redact(c.RadarrAPIKey))
	fmt.Fprintf(&b, "SONARR_URL=%s\n", c.SonarrURL)
	fmt.Fprintf(&b, "SONARR_API_KEY=%s\n", redact(c.SonarrAPIKey))
	fmt.Fprintf(&b, "INFLUX_URL=%s\n", c.InfluxURL)
	fmt.Fprintf(&b, "INFLUX_TOKEN=%s\n", redact(c.InfluxToken))
	fmt.Fprintf(&b, "INFLUX_ORG=%s\n", c.InfluxOrg)
	fmt.Fprintf(&b, "INFLUX_BUCKET=%s\n", c.InfluxBucket)
	fmt.Fprintf(&b, "SNAPSHOT_PATH=%s\n", c.SnapshotPath)
	fmt.Fprintf(&b, "SNAPSHOT_RETENTION=%d\n", c.SnapshotRetention)
	fmt.Fprintf(&b, "MAX_MEDIA_BYTES_DECREASE_PERCENT=%v\n", c.MaxMediaBytesDecreasePercent)
	fmt.Fprintf(&b, "MAX_MOVIE_DECREASE_PERCENT=%v\n", c.MaxMovieDecreasePercent)
	fmt.Fprintf(&b, "MAX_EPISODE_DECREASE_PERCENT=%v\n", c.MaxEpisodeDecreasePercent)
	fmt.Fprintf(&b, "MAX_FILES_REMOVED=%d\n", c.MaxFilesRemoved)
	fmt.Fprintf(&b, "MAX_UNMATCHED_PERCENT=%v\n", c.MaxUnmatchedPercent)
	fmt.Fprintf(&b, "RCLONE_REMOTE=%s\n", c.RcloneRemote)
	return b.String()
}

const (
	defaultTZ                           = "UTC"
	defaultSnapshotPath                 = "/data/snapshots"
	defaultSnapshotRetention            = 5
	defaultMaxMediaBytesDecreasePercent = 5
	defaultMaxMovieDecreasePercent      = 5
	defaultMaxEpisodeDecreasePercent    = 5
	defaultMaxFilesRemoved              = 100
	defaultMaxUnmatchedPercent          = 5
)

// Load builds a Config from environment variables read through getenv.
// It aggregates every problem it finds (missing required vars, unparseable
// values, out-of-range values) into a single error rather than stopping at
// the first one.
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	requireVar := func(name string) string {
		v := strings.TrimSpace(getenv(name))
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", name))
		}
		return v
	}

	cfg := Config{
		MediaMoviesPath: requireVar("MEDIA_MOVIES_PATH"),
		MediaTVPath:     requireVar("MEDIA_TV_PATH"),
		RadarrURL:       normalizeURL(requireVar("RADARR_URL")),
		RadarrAPIKey:    requireVar("RADARR_API_KEY"),
		SonarrURL:       normalizeURL(requireVar("SONARR_URL")),
		SonarrAPIKey:    requireVar("SONARR_API_KEY"),

		InfluxURL:    normalizeURL(getenv("INFLUX_URL")),
		InfluxToken:  getenv("INFLUX_TOKEN"),
		InfluxOrg:    getenv("INFLUX_ORG"),
		InfluxBucket: getenv("INFLUX_BUCKET"),

		RcloneRemote: getenv("RCLONE_REMOTE"),
	}

	cfg.TZ = withDefault(getenv("TZ"), defaultTZ)
	if _, err := time.LoadLocation(cfg.TZ); err != nil {
		errs = append(errs, fmt.Errorf("TZ: invalid timezone %q: %w", cfg.TZ, err))
	}

	cfg.SnapshotPath = withDefault(getenv("SNAPSHOT_PATH"), defaultSnapshotPath)

	cfg.SnapshotRetention = parseIntDefault(getenv, "SNAPSHOT_RETENTION", defaultSnapshotRetention, &errs)
	if cfg.SnapshotRetention < 1 {
		errs = append(errs, fmt.Errorf("SNAPSHOT_RETENTION: must be >= 1, got %d", cfg.SnapshotRetention))
	}

	cfg.MaxFilesRemoved = parseIntDefault(getenv, "MAX_FILES_REMOVED", defaultMaxFilesRemoved, &errs)
	if cfg.MaxFilesRemoved < 0 {
		errs = append(errs, fmt.Errorf("MAX_FILES_REMOVED: must be >= 0, got %d", cfg.MaxFilesRemoved))
	}

	cfg.MaxMediaBytesDecreasePercent = parsePercentDefault(getenv, "MAX_MEDIA_BYTES_DECREASE_PERCENT", defaultMaxMediaBytesDecreasePercent, &errs)
	cfg.MaxMovieDecreasePercent = parsePercentDefault(getenv, "MAX_MOVIE_DECREASE_PERCENT", defaultMaxMovieDecreasePercent, &errs)
	cfg.MaxEpisodeDecreasePercent = parsePercentDefault(getenv, "MAX_EPISODE_DECREASE_PERCENT", defaultMaxEpisodeDecreasePercent, &errs)
	cfg.MaxUnmatchedPercent = parsePercentDefault(getenv, "MAX_UNMATCHED_PERCENT", defaultMaxUnmatchedPercent, &errs)

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

func withDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func normalizeURL(v string) string {
	return strings.TrimSuffix(v, "/")
}

func parseIntDefault(getenv func(string) string, name string, def int, errs *[]error) int {
	raw := getenv(name)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s: invalid integer %q: %w", name, raw, err))
		return def
	}
	return n
}

func parsePercentDefault(getenv func(string) string, name string, def float64, errs *[]error) float64 {
	raw := getenv(name)
	if raw == "" {
		return def
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s: invalid number %q: %w", name, raw, err))
		return def
	}
	if n < 0 || n > 100 {
		*errs = append(*errs, fmt.Errorf("%s: must be between 0 and 100, got %v", name, n))
	}
	return n
}
