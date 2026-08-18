package config

import (
	"strings"
	"testing"
)

func fullEnv() map[string]string {
	return map[string]string{
		"TZ":                               "America/New_York",
		"MEDIA_MOVIES_PATH":                "/media/movies",
		"MEDIA_TV_PATH":                    "/media/tv",
		"RADARR_URL":                       "http://radarr:7878",
		"RADARR_API_KEY":                   "radarr-key",
		"SONARR_URL":                       "http://sonarr:8989",
		"SONARR_API_KEY":                   "sonarr-key",
		"INFLUX_URL":                       "http://influxdb:8086",
		"INFLUX_TOKEN":                     "influx-token",
		"INFLUX_ORG":                       "homelab",
		"INFLUX_BUCKET":                    "media",
		"SNAPSHOT_PATH":                    "/data/snapshots",
		"SNAPSHOT_RETENTION":               "5",
		"MAX_MEDIA_BYTES_DECREASE_PERCENT": "5",
		"MAX_MOVIE_DECREASE_PERCENT":       "5",
		"MAX_EPISODE_DECREASE_PERCENT":     "5",
		"MAX_FILES_REMOVED":                "100",
		"MAX_UNMATCHED_PERCENT":            "5",
		"RCLONE_REMOTE":                    "media-inventory:",
	}
}

func getenvFrom(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func TestLoad_AllSet_ReturnsFullyPopulatedConfig(t *testing.T) {
	cfg, err := Load(getenvFrom(fullEnv()))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if cfg.TZ != "America/New_York" {
		t.Errorf("TZ = %q, want America/New_York", cfg.TZ)
	}
	if cfg.MediaMoviesPath != "/media/movies" {
		t.Errorf("MediaMoviesPath = %q", cfg.MediaMoviesPath)
	}
	if cfg.MediaTVPath != "/media/tv" {
		t.Errorf("MediaTVPath = %q", cfg.MediaTVPath)
	}
	if cfg.RadarrURL != "http://radarr:7878" {
		t.Errorf("RadarrURL = %q", cfg.RadarrURL)
	}
	if cfg.RadarrAPIKey != "radarr-key" {
		t.Errorf("RadarrAPIKey = %q", cfg.RadarrAPIKey)
	}
	if cfg.SonarrURL != "http://sonarr:8989" {
		t.Errorf("SonarrURL = %q", cfg.SonarrURL)
	}
	if cfg.SonarrAPIKey != "sonarr-key" {
		t.Errorf("SonarrAPIKey = %q", cfg.SonarrAPIKey)
	}
	if cfg.InfluxURL != "http://influxdb:8086" {
		t.Errorf("InfluxURL = %q", cfg.InfluxURL)
	}
	if cfg.InfluxToken != "influx-token" {
		t.Errorf("InfluxToken = %q", cfg.InfluxToken)
	}
	if cfg.InfluxOrg != "homelab" {
		t.Errorf("InfluxOrg = %q", cfg.InfluxOrg)
	}
	if cfg.InfluxBucket != "media" {
		t.Errorf("InfluxBucket = %q", cfg.InfluxBucket)
	}
	if cfg.SnapshotPath != "/data/snapshots" {
		t.Errorf("SnapshotPath = %q", cfg.SnapshotPath)
	}
	if cfg.SnapshotRetention != 5 {
		t.Errorf("SnapshotRetention = %d, want 5", cfg.SnapshotRetention)
	}
	if cfg.MaxMediaBytesDecreasePercent != 5 {
		t.Errorf("MaxMediaBytesDecreasePercent = %v, want 5", cfg.MaxMediaBytesDecreasePercent)
	}
	if cfg.MaxMovieDecreasePercent != 5 {
		t.Errorf("MaxMovieDecreasePercent = %v, want 5", cfg.MaxMovieDecreasePercent)
	}
	if cfg.MaxEpisodeDecreasePercent != 5 {
		t.Errorf("MaxEpisodeDecreasePercent = %v, want 5", cfg.MaxEpisodeDecreasePercent)
	}
	if cfg.MaxFilesRemoved != 100 {
		t.Errorf("MaxFilesRemoved = %d, want 100", cfg.MaxFilesRemoved)
	}
	if cfg.MaxUnmatchedPercent != 5 {
		t.Errorf("MaxUnmatchedPercent = %v, want 5", cfg.MaxUnmatchedPercent)
	}
	if cfg.RcloneRemote != "media-inventory:" {
		t.Errorf("RcloneRemote = %q", cfg.RcloneRemote)
	}
}

func TestLoad_DefaultsAppliedWhenUnset(t *testing.T) {
	env := fullEnv()
	for _, k := range []string{
		"TZ", "SNAPSHOT_PATH", "SNAPSHOT_RETENTION",
		"MAX_MEDIA_BYTES_DECREASE_PERCENT", "MAX_MOVIE_DECREASE_PERCENT",
		"MAX_EPISODE_DECREASE_PERCENT", "MAX_FILES_REMOVED", "MAX_UNMATCHED_PERCENT",
	} {
		delete(env, k)
	}

	cfg, err := Load(getenvFrom(env))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if cfg.TZ != "UTC" {
		t.Errorf("TZ default = %q, want UTC", cfg.TZ)
	}
	if cfg.SnapshotPath != "/data/snapshots" {
		t.Errorf("SnapshotPath default = %q", cfg.SnapshotPath)
	}
	if cfg.SnapshotRetention != 5 {
		t.Errorf("SnapshotRetention default = %d, want 5", cfg.SnapshotRetention)
	}
	if cfg.MaxMediaBytesDecreasePercent != 5 {
		t.Errorf("MaxMediaBytesDecreasePercent default = %v, want 5", cfg.MaxMediaBytesDecreasePercent)
	}
	if cfg.MaxMovieDecreasePercent != 5 {
		t.Errorf("MaxMovieDecreasePercent default = %v, want 5", cfg.MaxMovieDecreasePercent)
	}
	if cfg.MaxEpisodeDecreasePercent != 5 {
		t.Errorf("MaxEpisodeDecreasePercent default = %v, want 5", cfg.MaxEpisodeDecreasePercent)
	}
	if cfg.MaxFilesRemoved != 100 {
		t.Errorf("MaxFilesRemoved default = %d, want 100", cfg.MaxFilesRemoved)
	}
	if cfg.MaxUnmatchedPercent != 5 {
		t.Errorf("MaxUnmatchedPercent default = %v, want 5", cfg.MaxUnmatchedPercent)
	}
}

func TestLoad_MissingRequiredVar_ErrorNamesVar(t *testing.T) {
	env := fullEnv()
	delete(env, "RADARR_URL")

	_, err := Load(getenvFrom(env))
	if err == nil {
		t.Fatal("Load() error = nil, want error naming RADARR_URL")
	}
	if !strings.Contains(err.Error(), "RADARR_URL") {
		t.Errorf("error = %q, want it to mention RADARR_URL", err.Error())
	}
}

func TestLoad_MissingSeveralRequiredVars_ErrorListsAll(t *testing.T) {
	env := fullEnv()
	delete(env, "RADARR_URL")
	delete(env, "SONARR_API_KEY")
	delete(env, "MEDIA_TV_PATH")

	_, err := Load(getenvFrom(env))
	if err == nil {
		t.Fatal("Load() error = nil, want aggregated error")
	}
	for _, want := range []string{"RADARR_URL", "SONARR_API_KEY", "MEDIA_TV_PATH"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to mention %s", err.Error(), want)
		}
	}
}

func TestLoad_SnapshotRetentionNotAnInteger_ErrorNamesVar(t *testing.T) {
	env := fullEnv()
	env["SNAPSHOT_RETENTION"] = "abc"

	_, err := Load(getenvFrom(env))
	if err == nil {
		t.Fatal("Load() error = nil, want parse error")
	}
	if !strings.Contains(err.Error(), "SNAPSHOT_RETENTION") {
		t.Errorf("error = %q, want it to mention SNAPSHOT_RETENTION", err.Error())
	}
}

func TestLoad_SnapshotRetentionNotPositive_ValidationError(t *testing.T) {
	for _, v := range []string{"0", "-1"} {
		env := fullEnv()
		env["SNAPSHOT_RETENTION"] = v

		_, err := Load(getenvFrom(env))
		if err == nil {
			t.Fatalf("Load() with SNAPSHOT_RETENTION=%s: error = nil, want validation error", v)
		}
		if !strings.Contains(err.Error(), "SNAPSHOT_RETENTION") {
			t.Errorf("SNAPSHOT_RETENTION=%s: error = %q, want it to mention SNAPSHOT_RETENTION", v, err.Error())
		}
	}
}

func TestLoad_MaxUnmatchedPercentOutOfRange_ValidationError(t *testing.T) {
	env := fullEnv()
	env["MAX_UNMATCHED_PERCENT"] = "150"

	_, err := Load(getenvFrom(env))
	if err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "MAX_UNMATCHED_PERCENT") {
		t.Errorf("error = %q, want it to mention MAX_UNMATCHED_PERCENT", err.Error())
	}
}

func TestLoad_TrailingSlashOnURLs_Normalized(t *testing.T) {
	env := fullEnv()
	env["RADARR_URL"] = "http://radarr:7878/"
	env["SONARR_URL"] = "http://sonarr:8989/"
	env["INFLUX_URL"] = "http://influxdb:8086/"

	cfg, err := Load(getenvFrom(env))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.RadarrURL != "http://radarr:7878" {
		t.Errorf("RadarrURL = %q, want no trailing slash", cfg.RadarrURL)
	}
	if cfg.SonarrURL != "http://sonarr:8989" {
		t.Errorf("SonarrURL = %q, want no trailing slash", cfg.SonarrURL)
	}
	if cfg.InfluxURL != "http://influxdb:8086" {
		t.Errorf("InfluxURL = %q, want no trailing slash", cfg.InfluxURL)
	}
}

func TestConfig_String_RedactsSecrets(t *testing.T) {
	cfg, err := Load(getenvFrom(fullEnv()))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	s := cfg.String()
	for _, secret := range []string{"radarr-key", "sonarr-key", "influx-token"} {
		if strings.Contains(s, secret) {
			t.Errorf("String() = %q, contains unredacted secret %q", s, secret)
		}
	}
	if !strings.Contains(s, "REDACTED") {
		t.Errorf("String() = %q, want it to show REDACTED for secrets", s)
	}
}

func TestConfig_OffsiteEnabled(t *testing.T) {
	env := fullEnv()
	cfg, err := Load(getenvFrom(env))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if !cfg.OffsiteEnabled() {
		t.Error("OffsiteEnabled() = false, want true when RCLONE_REMOTE set")
	}

	delete(env, "RCLONE_REMOTE")
	cfg, err = Load(getenvFrom(env))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.OffsiteEnabled() {
		t.Error("OffsiteEnabled() = true, want false when RCLONE_REMOTE unset")
	}
}

func TestLoad_InvalidTZ_Error(t *testing.T) {
	env := fullEnv()
	env["TZ"] = "Not/A_Real_Zone"

	_, err := Load(getenvFrom(env))
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid TZ")
	}
	if !strings.Contains(err.Error(), "TZ") {
		t.Errorf("error = %q, want it to mention TZ", err.Error())
	}
}

func TestLoad_PerformsNoIO(t *testing.T) {
	// Load must only call the injected getenv - proven by never touching the
	// real environment even when it differs from what getenv returns.
	t.Setenv("MEDIA_MOVIES_PATH", "/should/not/be/used")

	env := fullEnv()
	cfg, err := Load(getenvFrom(env))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.MediaMoviesPath != "/media/movies" {
		t.Errorf("MediaMoviesPath = %q, want value from injected getenv, not process env", cfg.MediaMoviesPath)
	}
}
