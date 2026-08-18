package influx

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func fullMetrics() Metrics {
	return Metrics{
		Movies:               4284,
		Series:               317,
		Episodes:             21506,
		MediaFiles:           25790,
		TotalBytes:           42648192382976,
		MoviesAdded:          1,
		MoviesRemoved:        0,
		SeriesAdded:          0,
		SeriesRemoved:        0,
		EpisodesAdded:        14,
		EpisodesRemoved:      0,
		FilesAdded:           15,
		FilesRemoved:         0,
		BytesAdded:           100000000000,
		BytesRemoved:         0,
		UnmatchedFiles:       3,
		ScanDurationSeconds:  42.8,
		SnapshotValid:        true,
		SnapshotWarning:      false,
		OffsiteUploadSuccess: true,
	}
}

func fullTags() Tags {
	return Tags{Host: "unraid", Job: "media-inventory", SchemaVersion: 1}
}

func TestEncode_GoldenLine(t *testing.T) {
	ts := time.Date(2026, 8, 16, 23, 0, 0, 0, time.UTC)
	got := Encode(fullMetrics(), fullTags(), ts)

	want := "media_inventory,host=unraid,job=media-inventory,schema_version=1 " +
		"movies=4284i,series=317i,episodes=21506i,media_files=25790i,total_bytes=42648192382976i," +
		"movies_added=1i,movies_removed=0i,series_added=0i,series_removed=0i," +
		"episodes_added=14i,episodes_removed=0i,files_added=15i,files_removed=0i," +
		"bytes_added=100000000000i,bytes_removed=0i,unmatched_files=3i,scan_duration_seconds=42.8," +
		"snapshot_valid=1i,snapshot_warning=0i,offsite_upload_success=1i " +
		"1786921200000000000"

	if got != want {
		t.Errorf("Encode() =\n%s\nwant:\n%s", got, want)
	}
}

func TestEncode_EveryExpectedFieldPresent(t *testing.T) {
	line := Encode(fullMetrics(), fullTags(), time.Now())
	fieldPart := strings.SplitN(line, " ", 3)[1]

	want := []string{
		"movies", "series", "episodes", "media_files", "total_bytes",
		"movies_added", "movies_removed", "series_added", "series_removed",
		"episodes_added", "episodes_removed", "files_added", "files_removed",
		"bytes_added", "bytes_removed", "unmatched_files", "scan_duration_seconds",
		"snapshot_valid", "snapshot_warning", "offsite_upload_success",
	}
	for _, key := range want {
		if !strings.Contains(fieldPart, key+"=") {
			t.Errorf("field %q missing from encoded line: %s", key, fieldPart)
		}
	}
	gotCount := strings.Count(fieldPart, "=")
	if gotCount != len(want) {
		t.Errorf("field count = %d, want %d (an omission or extra field slipped in)", gotCount, len(want))
	}
}

func TestEncode_IntegerFieldsSuffixedI_FloatIsNot(t *testing.T) {
	line := Encode(fullMetrics(), fullTags(), time.Now())
	fieldPart := strings.SplitN(line, " ", 3)[1]

	if !strings.Contains(fieldPart, "movies=4284i") {
		t.Errorf("integer field not suffixed 'i': %s", fieldPart)
	}
	if !strings.Contains(fieldPart, "scan_duration_seconds=42.8") {
		t.Errorf("float field malformed: %s", fieldPart)
	}
	if strings.Contains(fieldPart, "scan_duration_seconds=42.8i") {
		t.Error("scan_duration_seconds must not be suffixed 'i' - it is a float field")
	}
}

func TestEncode_BooleansRenderAsIntegers(t *testing.T) {
	m := fullMetrics()
	m.SnapshotValid = true
	m.SnapshotWarning = false
	m.OffsiteUploadSuccess = false
	line := Encode(m, fullTags(), time.Now())

	if !strings.Contains(line, "snapshot_valid=1i") {
		t.Errorf("snapshot_valid=true should render 1i: %s", line)
	}
	if !strings.Contains(line, "snapshot_warning=0i") {
		t.Errorf("snapshot_warning=false should render 0i: %s", line)
	}
	if !strings.Contains(line, "offsite_upload_success=0i") {
		t.Errorf("offsite_upload_success=false should render 0i: %s", line)
	}
}

func TestEncode_TagKeySet_ExactlyThreeNoHighCardinality(t *testing.T) {
	line := Encode(fullMetrics(), fullTags(), time.Now())
	tagPart := strings.SplitN(line, " ", 2)[0] // "media_inventory,host=...,job=...,schema_version=..."

	tagKeys := regexp.MustCompile(`(\w+)=`).FindAllStringSubmatch(tagPart, -1)
	got := make(map[string]bool)
	for _, m := range tagKeys {
		got[m[1]] = true
	}
	want := map[string]bool{"host": true, "job": true, "schema_version": true}
	if len(got) != len(want) {
		t.Fatalf("tag keys = %v, want exactly %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing tag %q", k)
		}
	}
}

func TestEncode_SpecialCharactersInTagValueEscaped(t *testing.T) {
	tags := Tags{Host: "un raid,host=x", Job: "media-inventory", SchemaVersion: 1}
	line := Encode(fullMetrics(), tags, time.Now())

	// The escaped value legitimately contains a literal space (preceded by
	// a backslash), so check the whole line rather than naively splitting
	// on " " - that would cut the tag section short right at the escape.
	if !strings.Contains(line, `host=un\ raid\,host\=x`) {
		t.Errorf("tag value not escaped in line: %s", line)
	}
}

func TestEncode_TagsSortedAlphabetically(t *testing.T) {
	line := Encode(fullMetrics(), fullTags(), time.Now())
	tagPart := strings.SplitN(line, " ", 2)[0]

	hostIdx := strings.Index(tagPart, "host=")
	jobIdx := strings.Index(tagPart, "job=")
	schemaIdx := strings.Index(tagPart, "schema_version=")
	if !(hostIdx < jobIdx && jobIdx < schemaIdx) {
		t.Errorf("tags not in sorted order: %s", tagPart)
	}
}

func TestEncode_TimestampIsNanosecondPrecisionFromInjectedClock(t *testing.T) {
	ts := time.Date(2026, 8, 16, 23, 0, 0, 123456789, time.UTC)
	line := Encode(fullMetrics(), fullTags(), ts)

	parts := strings.Fields(line)
	gotTS := parts[len(parts)-1]
	wantTS := "1786921200123456789"
	if gotTS != wantTS {
		t.Errorf("timestamp = %s, want %s", gotTS, wantTS)
	}
}
