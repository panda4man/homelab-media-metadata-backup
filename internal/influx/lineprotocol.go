package influx

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const measurement = "media_inventory"

// Encode renders m as one InfluxDB line-protocol point, tagged with tags
// and timestamped at ns nanosecond precision. Integer fields are suffixed
// "i"; booleans render as 1i/0i (not native true/false) so Grafana can
// sum/average them directly without a cast; scan_duration_seconds is the
// one plain float field.
func Encode(m Metrics, tags Tags, ts time.Time) string {
	tagPairs := []struct{ key, val string }{
		{"host", tags.Host},
		{"job", tags.Job},
		{"schema_version", strconv.Itoa(tags.SchemaVersion)},
	}
	sort.Slice(tagPairs, func(i, j int) bool { return tagPairs[i].key < tagPairs[j].key })

	var b strings.Builder
	b.WriteString(measurement)
	for _, p := range tagPairs {
		b.WriteByte(',')
		b.WriteString(escapeTag(p.key))
		b.WriteByte('=')
		b.WriteString(escapeTag(p.val))
	}
	b.WriteByte(' ')

	fields := []struct{ key, val string }{
		{"movies", intField(m.Movies)},
		{"series", intField(m.Series)},
		{"episodes", intField(m.Episodes)},
		{"media_files", intField(m.MediaFiles)},
		{"total_bytes", int64Field(m.TotalBytes)},
		{"movies_added", intField(m.MoviesAdded)},
		{"movies_removed", intField(m.MoviesRemoved)},
		{"series_added", intField(m.SeriesAdded)},
		{"series_removed", intField(m.SeriesRemoved)},
		{"episodes_added", intField(m.EpisodesAdded)},
		{"episodes_removed", intField(m.EpisodesRemoved)},
		{"files_added", intField(m.FilesAdded)},
		{"files_removed", intField(m.FilesRemoved)},
		{"bytes_added", int64Field(m.BytesAdded)},
		{"bytes_removed", int64Field(m.BytesRemoved)},
		{"unmatched_files", intField(m.UnmatchedFiles)},
		{"scan_duration_seconds", floatField(m.ScanDurationSeconds)},
		{"snapshot_valid", boolField(m.SnapshotValid)},
		{"snapshot_warning", boolField(m.SnapshotWarning)},
		{"offsite_upload_success", boolField(m.OffsiteUploadSuccess)},
	}
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(f.key)
		b.WriteByte('=')
		b.WriteString(f.val)
	}

	fmt.Fprintf(&b, " %d", ts.UnixNano())
	return b.String()
}

func intField(v int) string       { return strconv.Itoa(v) + "i" }
func int64Field(v int64) string   { return strconv.FormatInt(v, 10) + "i" }
func floatField(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func boolField(v bool) string {
	if v {
		return "1i"
	}
	return "0i"
}

var tagEscaper = strings.NewReplacer(",", `\,`, "=", `\=`, " ", `\ `)

func escapeTag(s string) string {
	return tagEscaper.Replace(s)
}
