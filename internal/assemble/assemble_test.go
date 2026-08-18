package assemble

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/filesystem"
	"github.com/panda4man/homelab-media-metadata-backup/internal/match"
	"github.com/panda4man/homelab-media-metadata-backup/internal/radarr"
	"github.com/panda4man/homelab-media-metadata-backup/internal/sonarr"
)

func mtime() time.Time { return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) }

func fullInput() Input {
	return Input{
		Hostname:    "unraid",
		GeneratedAt: time.Date(2026, 8, 16, 23, 0, 0, 0, time.UTC),
		MovieFiles: []filesystem.File{
			{Root: "movies", RelPath: "Inception (2010)/Inception.mkv", Bytes: 3481932841, ModTime: mtime()},
			{Root: "movies", RelPath: "Sample (1999)/Sample.mkv", Bytes: 1000, ModTime: mtime()},
			{Root: "movies", RelPath: "Orphan (2005)/Orphan.mkv", Bytes: 500, ModTime: mtime()},
		},
		RadarrMovies: []radarr.Movie{
			{Title: "Inception", Year: 2010, TMDBID: 27205, IMDbID: "tt1375666", Path: "/media/movies/Inception (2010)", RelativePath: "Inception.mkv", Bytes: 3481932841, HasFile: true},
			{Title: "Sample", Year: 1999, TMDBID: 42, Path: "/media/movies/Sample (1999)", RelativePath: "Sample.mkv", Bytes: 1000, HasFile: true},
		},
		TVFiles: []filesystem.File{
			{Root: "tv", RelPath: "Severance (2022)/Season 01/Severance - S01E01.mkv", Bytes: 5000000000, ModTime: mtime()},
			{Root: "tv", RelPath: "Untitled Show (2021)/Season 01/Untitled - S01E01.mkv", Bytes: 500, ModTime: mtime()},
		},
		Series: []match.SeriesWithEpisodes{
			{
				Series: sonarr.Series{Title: "Severance", Year: 2022, TVDBID: 371980, TMDBID: 95396, Path: "/media/tv/Severance (2022)"},
				Episodes: []sonarr.Episode{
					{Season: 1, Episode: 1, Title: "Good News About Hell", HasFile: true, RelativePath: "Season 01/Severance - S01E01.mkv", Bytes: 5000000000},
				},
			},
			{
				Series: sonarr.Series{Title: "Untitled Show", Year: 2021, TVDBID: 1, Path: "/media/tv/Untitled Show (2021)"},
				Episodes: []sonarr.Episode{
					{Season: 1, Episode: 1, Title: "Pilot", HasFile: true, RelativePath: "Season 01/Untitled - S01E01.mkv", Bytes: 500},
				},
			},
		},
	}
}

func TestBuild_FullFixture_ProducesExpectedSnapshot(t *testing.T) {
	snapshot, _ := Build(fullInput())

	if len(snapshot.Movies) != 2 {
		t.Fatalf("Movies = %v, want 2", snapshot.Movies)
	}
	if len(snapshot.Series) != 2 {
		t.Fatalf("Series = %v, want 2", snapshot.Series)
	}
	total := 0
	for _, s := range snapshot.Series {
		total += len(s.Episodes)
	}
	if total != 2 {
		t.Errorf("total episodes = %d, want 2", total)
	}
}

func TestBuild_Summary_ComputedCorrectly(t *testing.T) {
	snapshot, _ := Build(fullInput())

	if snapshot.Summary.Movies != 2 {
		t.Errorf("Summary.Movies = %d, want 2", snapshot.Summary.Movies)
	}
	if snapshot.Summary.Series != 2 {
		t.Errorf("Summary.Series = %d, want 2", snapshot.Summary.Series)
	}
	if snapshot.Summary.Episodes != 2 {
		t.Errorf("Summary.Episodes = %d, want 2", snapshot.Summary.Episodes)
	}
	if snapshot.Summary.MediaFiles != snapshot.Summary.Movies+snapshot.Summary.Episodes {
		t.Errorf("MediaFiles = %d, want Movies+Episodes", snapshot.Summary.MediaFiles)
	}
	wantBytes := int64(3481932841 + 1000 + 5000000000 + 500)
	if snapshot.Summary.TotalBytes != wantBytes {
		t.Errorf("TotalBytes = %d, want %d", snapshot.Summary.TotalBytes, wantBytes)
	}
}

func TestBuild_GeneratedAt_ComesFromInputNotWallClock(t *testing.T) {
	in := fullInput()
	before := time.Now()
	snapshot, _ := Build(in)
	after := time.Now()

	if !snapshot.GeneratedAt.Equal(in.GeneratedAt) {
		t.Errorf("GeneratedAt = %v, want input value %v", snapshot.GeneratedAt, in.GeneratedAt)
	}
	if snapshot.GeneratedAt.After(before) && snapshot.GeneratedAt.Before(after) {
		t.Error("GeneratedAt looks like it came from time.Now(), not the injected input")
	}
}

func TestBuild_Hostname_ComesFromInput(t *testing.T) {
	snapshot, _ := Build(fullInput())
	if snapshot.Hostname != "unraid" {
		t.Errorf("Hostname = %q, want unraid", snapshot.Hostname)
	}
}

func TestBuild_SchemaVersion_Is1(t *testing.T) {
	snapshot, _ := Build(fullInput())
	if snapshot.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", snapshot.SchemaVersion)
	}
}

func TestBuild_EmptyInput_ValidZeroSnapshotWithNonNilSlices(t *testing.T) {
	snapshot, stats := Build(Input{Hostname: "h", GeneratedAt: time.Now()})

	if snapshot.Summary.Movies != 0 || snapshot.Summary.Series != 0 || snapshot.Summary.Episodes != 0 {
		t.Errorf("Summary = %+v, want all zero", snapshot.Summary)
	}
	if stats.UnmatchedFiles != 0 {
		t.Errorf("UnmatchedFiles = %d, want 0", stats.UnmatchedFiles)
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if string(raw["movies"]) != "[]" {
		t.Errorf(`movies = %s, want "[]" not null`, raw["movies"])
	}
	if string(raw["series"]) != "[]" {
		t.Errorf(`series = %s, want "[]" not null`, raw["series"])
	}
}

func TestBuild_Stats_UnmatchedFiles_SumsMovieAndTVOrphans(t *testing.T) {
	in := fullInput() // includes one orphan movie file with no Radarr entry

	_, stats := Build(in)
	if stats.UnmatchedFiles != 1 {
		t.Errorf("UnmatchedFiles = %d, want 1 (the orphan movie file)", stats.UnmatchedFiles)
	}
}

func TestBuild_Deterministic_AcrossMultipleCalls(t *testing.T) {
	in := fullInput()

	first, _ := Build(in)
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	for i := 0; i < 5; i++ {
		got, _ := Build(in)
		gotJSON, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if string(gotJSON) != string(firstJSON) {
			t.Fatalf("run %d: Build() not deterministic:\nfirst: %s\ngot:   %s", i, firstJSON, gotJSON)
		}
	}
}

func TestBuild_DoesNotMutateInput(t *testing.T) {
	in := fullInput()
	wantMovieFiles := len(in.MovieFiles)
	wantTVFiles := len(in.TVFiles)
	wantRadarrMovies := len(in.RadarrMovies)
	wantSeries := len(in.Series)

	Build(in)

	if len(in.MovieFiles) != wantMovieFiles || len(in.TVFiles) != wantTVFiles ||
		len(in.RadarrMovies) != wantRadarrMovies || len(in.Series) != wantSeries {
		t.Error("Build() mutated its Input's slices")
	}
}
