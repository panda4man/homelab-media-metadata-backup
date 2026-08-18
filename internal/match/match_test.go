package match

import (
	"testing"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/filesystem"
	"github.com/panda4man/homelab-media-metadata-backup/internal/radarr"
	"github.com/panda4man/homelab-media-metadata-backup/internal/sonarr"
)

func mtime() time.Time { return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) }

func TestMatchMovies_FileMatchesRadarrEntry_MatchedWithTMDBID(t *testing.T) {
	files := []filesystem.File{
		{Root: "movies", RelPath: "Inception (2010)/Inception.mkv", Bytes: 1000, ModTime: mtime()},
	}
	movies := []radarr.Movie{
		{Title: "Inception", Year: 2010, TMDBID: 27205, IMDbID: "tt1375666", Path: "/media/movies/Inception (2010)", RelativePath: "Inception.mkv", Bytes: 1000, HasFile: true},
	}

	matched, unmatched := MatchMovies(files, movies)

	if len(matched) != 1 {
		t.Fatalf("matched = %v, want 1 entry", matched)
	}
	m := matched[0]
	if m.TMDBID != 27205 || m.Title != "Inception" || m.IMDbID != "tt1375666" {
		t.Errorf("matched[0] = %+v, want preserved Radarr metadata", m)
	}
	if m.Path != "Inception (2010)/Inception.mkv" {
		t.Errorf("Path = %q, want the walked file's relative path", m.Path)
	}
	if len(unmatched) != 0 {
		t.Errorf("unmatched = %v, want none", unmatched)
	}
}

func TestMatchMovies_RadarrMovieWithNoFile_AbsentFromResult(t *testing.T) {
	var files []filesystem.File // nothing on disk
	movies := []radarr.Movie{
		{Title: "Ghost", Year: 2020, TMDBID: 1, Path: "/media/movies/Ghost (2020)", HasFile: false},
	}

	matched, unmatched := MatchMovies(files, movies)

	if len(matched) != 0 {
		t.Errorf("matched = %v, want none - Radarr has no file for this movie", matched)
	}
	if len(unmatched) != 0 {
		t.Errorf("unmatched = %v, want none", unmatched)
	}
}

func TestMatchMovies_FileWithNoRadarrEntry_Unmatched(t *testing.T) {
	files := []filesystem.File{
		{Root: "movies", RelPath: "Orphan (2021)/Orphan.mkv", Bytes: 500, ModTime: mtime()},
	}
	var movies []radarr.Movie

	matched, unmatched := MatchMovies(files, movies)

	if len(matched) != 0 {
		t.Errorf("matched = %v, want none", matched)
	}
	if len(unmatched) != 1 || unmatched[0].RelPath != "Orphan (2021)/Orphan.mkv" {
		t.Errorf("unmatched = %v, want the orphan file", unmatched)
	}
}

func TestMatchMovies_PathNormalization_SeparatorsAndDotSlashStillMatch(t *testing.T) {
	files := []filesystem.File{
		{Root: "movies", RelPath: "Inception (2010)/Inception.mkv", Bytes: 1000, ModTime: mtime()},
	}
	movies := []radarr.Movie{
		{Title: "Inception", Year: 2010, TMDBID: 27205, Path: "/media/movies/Inception (2010)", RelativePath: "./Inception.mkv", HasFile: true},
	}

	matched, _ := MatchMovies(files, movies)
	if len(matched) != 1 {
		t.Fatalf("matched = %v, want 1 entry despite ./ prefix in Radarr's relativePath", matched)
	}
}

func TestMatchMovies_CaseInsensitiveFallback(t *testing.T) {
	files := []filesystem.File{
		{Root: "movies", RelPath: "Inception (2010)/INCEPTION.MKV", Bytes: 1000, ModTime: mtime()},
	}
	movies := []radarr.Movie{
		{Title: "Inception", Year: 2010, TMDBID: 27205, Path: "/media/movies/Inception (2010)", RelativePath: "Inception.mkv", HasFile: true},
	}

	matched, _ := MatchMovies(files, movies)
	if len(matched) != 1 {
		t.Fatalf("matched = %v, want 1 entry via case-insensitive fallback", matched)
	}
}

func TestMatchMovies_SameBasenameDifferentDirs_NotCrossMatched(t *testing.T) {
	files := []filesystem.File{
		{Root: "movies", RelPath: "Movie A (2001)/movie.mkv", Bytes: 111, ModTime: mtime()},
		{Root: "movies", RelPath: "Movie B (2002)/movie.mkv", Bytes: 222, ModTime: mtime()},
	}
	movies := []radarr.Movie{
		{Title: "Movie A", Year: 2001, TMDBID: 1, Path: "/media/movies/Movie A (2001)", RelativePath: "movie.mkv", HasFile: true},
		{Title: "Movie B", Year: 2002, TMDBID: 2, Path: "/media/movies/Movie B (2002)", RelativePath: "movie.mkv", HasFile: true},
	}

	matched, unmatched := MatchMovies(files, movies)
	if len(matched) != 2 {
		t.Fatalf("matched = %v, want 2 entries", matched)
	}
	if len(unmatched) != 0 {
		t.Errorf("unmatched = %v, want none", unmatched)
	}
	for _, m := range matched {
		if m.Title == "Movie A" && m.Bytes != 111 {
			t.Errorf("Movie A Bytes = %d, want 111 (its own file, not Movie B's)", m.Bytes)
		}
		if m.Title == "Movie B" && m.Bytes != 222 {
			t.Errorf("Movie B Bytes = %d, want 222 (its own file, not Movie A's)", m.Bytes)
		}
	}
}

func TestMatchEpisodes_FileMatchesSeriesAndEpisode_CarriesEpisodeTitle(t *testing.T) {
	files := []filesystem.File{
		{Root: "tv", RelPath: "Severance (2022)/Season 01/Severance - S01E01.mkv", Bytes: 3481932841, ModTime: mtime()},
	}
	seriesList := []SeriesWithEpisodes{
		{
			Series: sonarr.Series{ID: 1, Title: "Severance", Year: 2022, TVDBID: 371980, TMDBID: 95396, Path: "/media/tv/Severance (2022)"},
			Episodes: []sonarr.Episode{
				{Season: 1, Episode: 1, Title: "Good News About Hell", HasFile: true, RelativePath: "Season 01/Severance - S01E01.mkv", Bytes: 3481932841},
			},
		},
	}

	matched, unmatched := MatchEpisodes(files, seriesList)

	if len(matched) != 1 {
		t.Fatalf("matched = %v, want 1 series", matched)
	}
	s := matched[0]
	if s.TVDBID != 371980 || s.Title != "Severance" {
		t.Errorf("matched[0] series = %+v, want preserved Sonarr metadata", s)
	}
	if len(s.Episodes) != 1 || s.Episodes[0].Title != "Good News About Hell" {
		t.Errorf("episodes = %v, want the matched episode with its title", s.Episodes)
	}
	if len(unmatched) != 0 {
		t.Errorf("unmatched = %v, want none", unmatched)
	}
}

func TestMatchEpisodes_HasFileButAbsentOnDisk_Omitted(t *testing.T) {
	var files []filesystem.File // nothing on disk
	seriesList := []SeriesWithEpisodes{
		{
			Series: sonarr.Series{Title: "Severance", TVDBID: 1, Path: "/media/tv/Severance"},
			Episodes: []sonarr.Episode{
				{Season: 1, Episode: 1, HasFile: true, RelativePath: "S01E01.mkv"},
			},
		},
	}

	matched, _ := MatchEpisodes(files, seriesList)
	if len(matched) != 0 {
		t.Errorf("matched = %v, want none - the file Sonarr thinks exists is not on disk", matched)
	}
}

func TestMatchEpisodes_ExtraFileInSeriesDir_UnmatchedButSeriesStillReported(t *testing.T) {
	files := []filesystem.File{
		{Root: "tv", RelPath: "Severance (2022)/Season 01/Severance - S01E01.mkv", Bytes: 100, ModTime: mtime()},
		{Root: "tv", RelPath: "Severance (2022)/Season 01/Severance - S01E01-sample.mkv", Bytes: 5, ModTime: mtime()},
	}
	seriesList := []SeriesWithEpisodes{
		{
			Series: sonarr.Series{Title: "Severance", TVDBID: 1, Path: "/media/tv/Severance (2022)"},
			Episodes: []sonarr.Episode{
				{Season: 1, Episode: 1, Title: "Good News About Hell", HasFile: true, RelativePath: "Season 01/Severance - S01E01.mkv", Bytes: 100},
			},
		},
	}

	matched, unmatched := MatchEpisodes(files, seriesList)
	if len(matched) != 1 || len(matched[0].Episodes) != 1 {
		t.Fatalf("matched = %v, want the series with its one real episode", matched)
	}
	if len(unmatched) != 1 || unmatched[0].RelPath != "Severance (2022)/Season 01/Severance - S01E01-sample.mkv" {
		t.Errorf("unmatched = %v, want the stray sample file", unmatched)
	}
}

func TestMatchEpisodes_SeriesWithZeroMatchedEpisodes_Excluded(t *testing.T) {
	var files []filesystem.File
	seriesList := []SeriesWithEpisodes{
		{
			Series: sonarr.Series{Title: "All Gone", TVDBID: 1, Path: "/media/tv/All Gone"},
			Episodes: []sonarr.Episode{
				{Season: 1, Episode: 1, HasFile: false},
			},
		},
	}

	matched, unmatched := MatchEpisodes(files, seriesList)
	if len(matched) != 0 {
		t.Errorf("matched = %v, want none - series has zero episodes with files", matched)
	}
	if len(unmatched) != 0 {
		t.Errorf("unmatched = %v, want none", unmatched)
	}
}

func TestMatchEpisodes_MultiEpisodeFile_BothCreditedToSameFile(t *testing.T) {
	files := []filesystem.File{
		{Root: "tv", RelPath: "Show (2020)/S01E03-E04.mkv", Bytes: 6000000000, ModTime: mtime()},
	}
	seriesList := []SeriesWithEpisodes{
		{
			Series: sonarr.Series{Title: "Show", TVDBID: 1, Path: "/media/tv/Show (2020)"},
			Episodes: []sonarr.Episode{
				{Season: 1, Episode: 3, Title: "Part 1", HasFile: true, RelativePath: "S01E03-E04.mkv", Bytes: 6000000000},
				{Season: 1, Episode: 4, Title: "Part 2", HasFile: true, RelativePath: "S01E03-E04.mkv", Bytes: 6000000000},
			},
		},
	}

	matched, unmatched := MatchEpisodes(files, seriesList)
	if len(matched) != 1 || len(matched[0].Episodes) != 2 {
		t.Fatalf("matched = %v, want one series with both episodes", matched)
	}
	// Only one physical file exists; it is consumed once and credited to
	// both episode entries, not left over as a duplicate/unmatched file.
	if len(unmatched) != 0 {
		t.Errorf("unmatched = %v, want none - the single file backs both episodes", unmatched)
	}
}

func TestUnmatchedPercent_ZeroTotal_ReturnsZero(t *testing.T) {
	if got := UnmatchedPercent(0, 0); got != 0 {
		t.Errorf("UnmatchedPercent(0, 0) = %v, want 0", got)
	}
}

func TestUnmatchedPercent_ComputesRatio(t *testing.T) {
	if got := UnmatchedPercent(5, 20); got != 25 {
		t.Errorf("UnmatchedPercent(5, 20) = %v, want 25", got)
	}
}

func TestMatchMovies_EveryWalkedFileAccountedForExactlyOnce(t *testing.T) {
	files := []filesystem.File{
		{Root: "movies", RelPath: "A (2001)/a.mkv", Bytes: 1, ModTime: mtime()},
		{Root: "movies", RelPath: "B (2002)/b.mkv", Bytes: 2, ModTime: mtime()},
		{Root: "movies", RelPath: "Orphan (2003)/o.mkv", Bytes: 3, ModTime: mtime()},
	}
	movies := []radarr.Movie{
		{Title: "A", Year: 2001, TMDBID: 1, Path: "/media/movies/A (2001)", RelativePath: "a.mkv", HasFile: true},
		{Title: "B", Year: 2002, TMDBID: 2, Path: "/media/movies/B (2002)", RelativePath: "b.mkv", HasFile: true},
	}

	matched, unmatched := MatchMovies(files, movies)

	seen := map[string]bool{}
	for _, m := range matched {
		if seen[m.Path] {
			t.Fatalf("path %q matched more than once", m.Path)
		}
		seen[m.Path] = true
	}
	for _, u := range unmatched {
		if seen[u.RelPath] {
			t.Fatalf("path %q appears in both matched and unmatched", u.RelPath)
		}
		seen[u.RelPath] = true
	}
	if len(seen) != len(files) {
		t.Errorf("accounted for %d paths, want %d (every walked file exactly once)", len(seen), len(files))
	}
}

func TestMatchMovies_Deterministic(t *testing.T) {
	files := []filesystem.File{
		{Root: "movies", RelPath: "Z (2001)/z.mkv", Bytes: 1, ModTime: mtime()},
		{Root: "movies", RelPath: "A (2002)/a.mkv", Bytes: 2, ModTime: mtime()},
	}
	movies := []radarr.Movie{
		{Title: "Z", Year: 2001, TMDBID: 1, Path: "/media/movies/Z (2001)", RelativePath: "z.mkv", HasFile: true},
		{Title: "A", Year: 2002, TMDBID: 2, Path: "/media/movies/A (2002)", RelativePath: "a.mkv", HasFile: true},
	}

	matched, _ := MatchMovies(files, movies)
	if len(matched) != 2 || matched[0].Title != "A" || matched[1].Title != "Z" {
		t.Errorf("matched = %v, want sorted by title (A before Z)", matched)
	}
}
