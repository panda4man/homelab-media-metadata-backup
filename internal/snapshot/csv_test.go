package snapshot

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/inventory"
)

// csvFixtureMovies exercises header/row mapping, RFC 4180 quoting (comma,
// quote), UTF-8 without BOM, an empty optional field, and a byte count
// beyond math.MaxInt32.
func csvFixtureMovies() []inventory.Movie {
	return []inventory.Movie{
		{Title: "Se7en", Year: 1995, TMDBID: 807, IMDbID: "tt0114369", Path: "Se7en (1995)/Se7en.mkv", Bytes: 3481932841},
		{Title: "Movie, With Comma", Year: 2000, TMDBID: 1, IMDbID: "", Path: "Movie, With Comma (2000)/movie.mkv", Bytes: 100},
		{Title: `Movie "Quoted" Title`, Year: 2001, TMDBID: 2, IMDbID: "tt1", Path: "Quoted (2001)/movie.mkv", Bytes: 200},
		{Title: "Café", Year: 2002, TMDBID: 3, IMDbID: "tt2", Path: "Cafe (2002)/movie.mkv", Bytes: 300},
	}
}

func csvFixtureSeries() []inventory.Series {
	return []inventory.Series{
		{
			Title: "Severance", Year: 2022, TVDBID: 371980,
			Episodes: []inventory.Episode{
				{Season: 1, Episode: 1, Title: "Good News About Hell", Path: "Severance (2022)/Season 01/S01E01.mkv", Bytes: 5000000000},
				{Season: 1, Episode: 2, Title: "Half Loop, Half Truth", Path: "Severance (2022)/Season 01/S01E02.mkv", Bytes: 4000000000},
			},
		},
	}
}

func fixedNow() time.Time { return time.Date(2026, 8, 16, 23, 0, 0, 0, time.UTC) }

func TestWriteMoviesCSV_MatchesGoldenFile(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteMoviesCSV(dir, csvFixtureMovies(), fixedNow())
	if err != nil {
		t.Fatalf("WriteMoviesCSV() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	want, err := os.ReadFile("testdata/movies_golden.csv")
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("movies CSV does not match golden.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteEpisodesCSV_MatchesGoldenFile(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteEpisodesCSV(dir, csvFixtureSeries(), fixedNow())
	if err != nil {
		t.Fatalf("WriteEpisodesCSV() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	want, err := os.ReadFile("testdata/episodes_golden.csv")
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("episodes CSV does not match golden.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteMoviesCSV_HeaderExact(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteMoviesCSV(dir, nil, fixedNow())
	if err != nil {
		t.Fatalf("WriteMoviesCSV() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	header := strings.SplitN(string(data), "\n", 2)[0]
	if header != "title,year,tmdb_id,imdb_id,path,bytes" {
		t.Errorf("header = %q, want exact spec header", header)
	}
}

func TestWriteEpisodesCSV_HeaderExact(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteEpisodesCSV(dir, nil, fixedNow())
	if err != nil {
		t.Fatalf("WriteEpisodesCSV() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	header := strings.SplitN(string(data), "\n", 2)[0]
	if header != "series_title,year,tvdb_id,season,episode,episode_title,path,bytes" {
		t.Errorf("header = %q, want exact spec header", header)
	}
}

func TestWriteMoviesCSV_EmptyInventory_HeaderOnlyNoError(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteMoviesCSV(dir, nil, fixedNow())
	if err != nil {
		t.Fatalf("WriteMoviesCSV() error = %v, want nil for empty inventory", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("lines = %v, want header-only", lines)
	}
}

func TestWriteMoviesCSV_BytesAsPlainInteger(t *testing.T) {
	dir := t.TempDir()
	movies := []inventory.Movie{{Title: "Big", Year: 2020, TMDBID: 1, Path: "p", Bytes: 3481932841}}
	path, err := WriteMoviesCSV(dir, movies, fixedNow())
	if err != nil {
		t.Fatalf("WriteMoviesCSV() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "3481932841") {
		t.Errorf("csv = %q, want plain integer 3481932841, no scientific notation", data)
	}
	if strings.ContainsAny(string(data), "eE+") && strings.Contains(string(data), "e+") {
		t.Errorf("csv = %q, looks like scientific notation", data)
	}
}

func TestWriteMoviesCSV_MissingIMDbID_EmptyField(t *testing.T) {
	dir := t.TempDir()
	movies := []inventory.Movie{{Title: "No IMDb", Year: 2020, TMDBID: 1, Path: "p", Bytes: 1}}
	path, err := WriteMoviesCSV(dir, movies, fixedNow())
	if err != nil {
		t.Fatalf("WriteMoviesCSV() error = %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if len(records) != 2 || records[1][3] != "" {
		t.Errorf("records = %v, want row[1].imdb_id column empty", records)
	}
}

func TestWriteMoviesCSV_AtomicNoTmpFilesRemain(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteMoviesCSV(dir, csvFixtureMovies(), fixedNow()); err != nil {
		t.Fatalf("WriteMoviesCSV() error = %v", err)
	}
	if _, err := WriteEpisodesCSV(dir, csvFixtureSeries(), fixedNow()); err != nil {
		t.Fatalf("WriteEpisodesCSV() error = %v", err)
	}
	noTmpFilesRemain(t, dir)
}

func TestCSVFilenames(t *testing.T) {
	dir := t.TempDir()
	moviesPath, err := WriteMoviesCSV(dir, nil, fixedNow())
	if err != nil {
		t.Fatalf("WriteMoviesCSV() error = %v", err)
	}
	if got, want := filepath.Base(moviesPath), "media-inventory-2026-08-16-movies.csv"; got != want {
		t.Errorf("movies filename = %q, want %q", got, want)
	}

	episodesPath, err := WriteEpisodesCSV(dir, nil, fixedNow())
	if err != nil {
		t.Fatalf("WriteEpisodesCSV() error = %v", err)
	}
	if got, want := filepath.Base(episodesPath), "media-inventory-2026-08-16-episodes.csv"; got != want {
		t.Errorf("episodes filename = %q, want %q", got, want)
	}
}
