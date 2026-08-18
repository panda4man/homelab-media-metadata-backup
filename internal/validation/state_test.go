package validation

import "testing"

func checksWith(triggeredIDs ...string) []Check {
	triggered := make(map[string]bool, len(triggeredIDs))
	for _, id := range triggeredIDs {
		triggered[id] = true
	}
	var out []Check
	for _, id := range []string{
		"total_bytes_decrease", "movie_count_decrease", "episode_count_decrease",
		"files_removed_threshold", "unmatched_percent_threshold",
		"sonarr_unreachable", "radarr_unreachable",
		"media_root_inaccessible", "media_root_empty", "scan_incomplete",
	} {
		out = append(out, Check{ID: id, Triggered: triggered[id]})
	}
	return out
}

func TestResolve_NothingTriggered_Valid(t *testing.T) {
	if got := Resolve(checksWith()); got != StateValid {
		t.Errorf("Resolve() = %v, want StateValid", got)
	}
}

func TestResolve_OnlyWarningLevelChecks_Warning(t *testing.T) {
	// The spec's own example: a large but plausible change is a warning,
	// explicitly NOT a failure.
	got := Resolve(checksWith("files_removed_threshold", "total_bytes_decrease"))
	if got != StateWarning {
		t.Errorf("Resolve() = %v, want StateWarning", got)
	}
}

func TestResolve_RadarrUnreachable_Failed(t *testing.T) {
	if got := Resolve(checksWith("radarr_unreachable")); got != StateFailed {
		t.Errorf("Resolve() = %v, want StateFailed", got)
	}
}

func TestResolve_SonarrUnreachable_Failed(t *testing.T) {
	if got := Resolve(checksWith("sonarr_unreachable")); got != StateFailed {
		t.Errorf("Resolve() = %v, want StateFailed", got)
	}
}

func TestResolve_MediaRootInaccessible_Failed(t *testing.T) {
	if got := Resolve(checksWith("media_root_inaccessible")); got != StateFailed {
		t.Errorf("Resolve() = %v, want StateFailed", got)
	}
}

func TestResolve_MediaRootEmpty_Failed(t *testing.T) {
	if got := Resolve(checksWith("media_root_empty")); got != StateFailed {
		t.Errorf("Resolve() = %v, want StateFailed", got)
	}
}

func TestResolve_ScanIncomplete_Failed(t *testing.T) {
	if got := Resolve(checksWith("scan_incomplete")); got != StateFailed {
		t.Errorf("Resolve() = %v, want StateFailed", got)
	}
}

func TestResolve_WarningAndFatalBothPresent_FailedWins(t *testing.T) {
	got := Resolve(checksWith("total_bytes_decrease", "radarr_unreachable"))
	if got != StateFailed {
		t.Errorf("Resolve() = %v, want StateFailed - a fatal condition always wins over a warning", got)
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		s    State
		want string
	}{
		{StateValid, "valid"},
		{StateWarning, "warning"},
		{StateFailed, "failed"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}
