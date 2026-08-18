package inventory

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// fixtureSnapshot builds a fully-populated Snapshot exercising every field,
// including a non-UTC GeneratedAt location and a byte count beyond
// math.MaxInt32, so marshal/unmarshal tests have real edges to check.
func fixtureSnapshot() Snapshot {
	edt := time.FixedZone("EDT", -4*3600)
	s := NewSnapshot(
		time.Date(2026, 8, 16, 23, 0, 0, 0, edt),
		"unraid",
		[]Movie{
			{
				Title:  "Inception",
				Year:   2010,
				TMDBID: 27205,
				IMDbID: "tt1375666",
				Dir:    "Movies/Inception (2010)",
				Path:   "Movies/Inception (2010)/Inception.mkv",
				Bytes:  3481932841,
				MTime:  time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
			},
			{
				Title:  "Sample",
				Year:   1999,
				TMDBID: 42,
				Dir:    "Movies/Sample (1999)",
				Path:   "Movies/Sample (1999)/Sample.mkv",
				Bytes:  1000,
				MTime:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		[]Series{
			{
				Title:  "Severance",
				Year:   2022,
				TVDBID: 371980,
				TMDBID: 95396,
				Dir:    "TV/Severance (2022)",
				Episodes: []Episode{
					{
						Season:  1,
						Episode: 1,
						Title:   "Good News About Hell",
						Path:    "TV/Severance (2022)/Season 01/Severance - S01E01.mkv",
						Bytes:   5000000000, // beyond math.MaxInt32
						MTime:   time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC),
					},
				},
			},
			{
				Title:  "Untitled Show",
				Year:   2021,
				TVDBID: 1,
				Dir:    "TV/Untitled Show (2021)",
				Episodes: []Episode{
					{
						Season:  1,
						Episode: 1,
						Title:   "Pilot",
						Path:    "TV/Untitled Show (2021)/Season 01/Untitled - S01E01.mkv",
						Bytes:   500,
						MTime:   time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
					},
				},
			},
		},
	)
	return s
}

func TestSnapshot_Marshal_MatchesGoldenFile(t *testing.T) {
	got, err := json.MarshalIndent(fixtureSnapshot(), "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	got = append(got, '\n')

	want, err := os.ReadFile("testdata/snapshot_golden.json")
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("marshaled snapshot does not match golden file.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestSnapshot_Unmarshal_GoldenFile_DeepEqualsFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/snapshot_golden.json")
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}

	var got Snapshot
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := fixtureSnapshot()
	want.GeneratedAt = want.GeneratedAt.UTC() // marshal forces UTC; unmarshal reads it back as UTC

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unmarshaled snapshot mismatch (-want +got):\n%s", diff)
	}
}

func TestSnapshot_RoundTrip_Stable(t *testing.T) {
	original := fixtureSnapshot()

	first, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("first Marshal() error = %v", err)
	}
	var decoded Snapshot
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	second, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("second Marshal() error = %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("round trip not stable:\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestNewSnapshot_AlwaysSetsSchemaVersion1(t *testing.T) {
	s := NewSnapshot(time.Now(), "host", nil, nil)
	if s.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", s.SchemaVersion)
	}
}

func TestSnapshot_LargeByteCount_SurvivesRoundTrip(t *testing.T) {
	s := fixtureSnapshot()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded Snapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	got := decoded.Series[0].Episodes[0].Bytes
	want := int64(5000000000)
	if got != want {
		t.Errorf("Bytes = %d, want %d", got, want)
	}
}

func TestMovie_OptionalFields_OmittedWhenEmpty(t *testing.T) {
	m := Movie{Title: "Sample", Year: 1999, TMDBID: 42, Dir: "d", Path: "p", Bytes: 1}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, present := raw["imdb_id"]; present {
		t.Errorf("imdb_id present in %s, want omitted", data)
	}
}

func TestMovie_OptionalFields_PresentWhenSet(t *testing.T) {
	m := Movie{Title: "Inception", Year: 2010, TMDBID: 27205, IMDbID: "tt1375666", Dir: "d", Path: "p", Bytes: 1}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got, present := raw["imdb_id"]; !present || got != "tt1375666" {
		t.Errorf("imdb_id = %v (present=%v), want tt1375666", got, present)
	}
}

func TestSeries_OptionalTMDBID_OmittedWhenEmpty(t *testing.T) {
	s := Series{Title: "Untitled Show", Year: 2021, TVDBID: 1, Dir: "d"}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, present := raw["tmdb_id"]; present {
		t.Errorf("tmdb_id present in %s, want omitted", data)
	}
}

func TestSnapshot_GeneratedAt_SerializesAsUTC(t *testing.T) {
	s := fixtureSnapshot() // constructed with EDT (-4:00) location
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	got, _ := raw["generated_at"].(string)
	want := "2026-08-17T03:00:00Z" // 23:00 EDT (-04:00) == 03:00 UTC next day
	if got != want {
		t.Errorf("generated_at = %q, want %q", got, want)
	}
}

func TestSnapshot_Unmarshal_UnknownField_DoesNotError(t *testing.T) {
	data, err := os.ReadFile("testdata/snapshot_golden.json")
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	raw["a_future_field_from_schema_2"] = "some new thing"
	withExtra, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var s Snapshot
	if err := json.Unmarshal(withExtra, &s); err != nil {
		t.Errorf("Unmarshal() with unknown field error = %v, want nil", err)
	}
}

func TestSnapshot_Unmarshal_UnsupportedSchemaVersion_ReturnsTypedError(t *testing.T) {
	data, err := os.ReadFile("testdata/snapshot_golden.json")
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	raw["schema_version"] = 2
	bumped, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var s Snapshot
	err = json.Unmarshal(bumped, &s)
	if !errors.Is(err, ErrUnsupportedSchema) {
		t.Errorf("Unmarshal() error = %v, want ErrUnsupportedSchema", err)
	}
}

func TestSummary_Recompute_DerivesCountersFromSlices(t *testing.T) {
	s := fixtureSnapshot()
	s.Summary = Summary{} // wipe, then recompute must rebuild it
	s.Summary.Recompute(s.Movies, s.Series)

	if s.Summary.Movies != 2 {
		t.Errorf("Movies = %d, want 2", s.Summary.Movies)
	}
	if s.Summary.Series != 2 {
		t.Errorf("Series = %d, want 2", s.Summary.Series)
	}
	if s.Summary.Episodes != 2 {
		t.Errorf("Episodes = %d, want 2", s.Summary.Episodes)
	}
	if s.Summary.MediaFiles != s.Summary.Movies+s.Summary.Episodes {
		t.Errorf("MediaFiles = %d, want Movies+Episodes = %d", s.Summary.MediaFiles, s.Summary.Movies+s.Summary.Episodes)
	}
	wantBytes := int64(3481932841 + 1000 + 5000000000 + 500)
	if s.Summary.TotalBytes != wantBytes {
		t.Errorf("TotalBytes = %d, want %d", s.Summary.TotalBytes, wantBytes)
	}
}
