package validation

import (
	"math"
	"testing"

	"github.com/panda4man/homelab-media-metadata-backup/internal/inventory"
)

func snap(movies []inventory.Movie, series []inventory.Series) inventory.Snapshot {
	var s inventory.Snapshot
	s.Movies = movies
	s.Series = series
	s.Summary.Recompute(movies, series)
	return s
}

func movie(tmdbID int, title string, year int, bytes int64) inventory.Movie {
	return inventory.Movie{Title: title, Year: year, TMDBID: tmdbID, Bytes: bytes}
}

func seriesWith(tvdbID int, title string, year int, episodes ...inventory.Episode) inventory.Series {
	return inventory.Series{Title: title, Year: year, TVDBID: tvdbID, Episodes: episodes}
}

func ep(season, episode int, bytes int64) inventory.Episode {
	return inventory.Episode{Season: season, Episode: episode, Bytes: bytes}
}

func TestCompare_IdenticalSnapshots_AllZeroDiff(t *testing.T) {
	s := snap(
		[]inventory.Movie{movie(1, "A", 2000, 100)},
		[]inventory.Series{seriesWith(1, "S", 2000, ep(1, 1, 50))},
	)

	d := Compare(s, true, s)

	if d.MoviesAdded != 0 || d.MoviesRemoved != 0 || d.SeriesAdded != 0 || d.SeriesRemoved != 0 ||
		d.EpisodesAdded != 0 || d.EpisodesRemoved != 0 || d.FilesAdded != 0 || d.FilesRemoved != 0 ||
		d.BytesAdded != 0 || d.BytesRemoved != 0 {
		t.Errorf("d = %+v, want all-zero diff", d)
	}
	if d.TotalSizeChangePercent != 0 {
		t.Errorf("TotalSizeChangePercent = %v, want 0", d.TotalSizeChangePercent)
	}
}

func TestCompare_MovieAdded(t *testing.T) {
	prev := snap([]inventory.Movie{movie(1, "A", 2000, 100)}, nil)
	cur := snap([]inventory.Movie{movie(1, "A", 2000, 100), movie(2, "B", 2001, 200)}, nil)

	d := Compare(prev, true, cur)

	if d.MoviesAdded != 1 || d.MoviesRemoved != 0 {
		t.Errorf("MoviesAdded/Removed = %d/%d, want 1/0", d.MoviesAdded, d.MoviesRemoved)
	}
	if d.FilesAdded != 1 {
		t.Errorf("FilesAdded = %d, want 1", d.FilesAdded)
	}
	if d.BytesAdded != 200 {
		t.Errorf("BytesAdded = %d, want 200", d.BytesAdded)
	}
}

func TestCompare_MovieRemoved(t *testing.T) {
	prev := snap([]inventory.Movie{movie(1, "A", 2000, 100), movie(2, "B", 2001, 200)}, nil)
	cur := snap([]inventory.Movie{movie(1, "A", 2000, 100)}, nil)

	d := Compare(prev, true, cur)

	if d.MoviesRemoved != 1 || d.MoviesAdded != 0 {
		t.Errorf("MoviesAdded/Removed = %d/%d, want 0/1", d.MoviesAdded, d.MoviesRemoved)
	}
	if d.FilesRemoved != 1 {
		t.Errorf("FilesRemoved = %d, want 1", d.FilesRemoved)
	}
	if d.BytesRemoved != 200 {
		t.Errorf("BytesRemoved = %d, want 200", d.BytesRemoved)
	}
}

func TestCompare_MovieReplacedWithLargerFile_SameTMDBID_NoAddRemoveJustBytes(t *testing.T) {
	prev := snap([]inventory.Movie{movie(1, "A", 2000, 100)}, nil)
	cur := snap([]inventory.Movie{movie(1, "A", 2000, 500)}, nil)

	d := Compare(prev, true, cur)

	if d.MoviesAdded != 0 || d.MoviesRemoved != 0 {
		t.Errorf("MoviesAdded/Removed = %d/%d, want 0/0 (same identity, just re-encoded)", d.MoviesAdded, d.MoviesRemoved)
	}
	if d.BytesAdded != 400 || d.BytesRemoved != 0 {
		t.Errorf("BytesAdded/Removed = %d/%d, want 400/0", d.BytesAdded, d.BytesRemoved)
	}
}

func TestCompare_EpisodeAddedToExistingSeries_SeriesNotCountedAsAdded(t *testing.T) {
	prev := snap(nil, []inventory.Series{seriesWith(1, "S", 2000, ep(1, 1, 50))})
	cur := snap(nil, []inventory.Series{seriesWith(1, "S", 2000, ep(1, 1, 50), ep(1, 2, 60))})

	d := Compare(prev, true, cur)

	if d.EpisodesAdded != 1 {
		t.Errorf("EpisodesAdded = %d, want 1", d.EpisodesAdded)
	}
	if d.SeriesAdded != 0 {
		t.Errorf("SeriesAdded = %d, want 0 - the series already existed", d.SeriesAdded)
	}
}

func TestCompare_WholeSeriesRemoved_CountsSeriesAndAllItsEpisodes(t *testing.T) {
	prev := snap(nil, []inventory.Series{seriesWith(1, "S", 2000, ep(1, 1, 50), ep(1, 2, 60))})
	cur := snap(nil, nil)

	d := Compare(prev, true, cur)

	if d.SeriesRemoved != 1 {
		t.Errorf("SeriesRemoved = %d, want 1", d.SeriesRemoved)
	}
	if d.EpisodesRemoved != 2 {
		t.Errorf("EpisodesRemoved = %d, want 2", d.EpisodesRemoved)
	}
	if d.BytesRemoved != 110 {
		t.Errorf("BytesRemoved = %d, want 110", d.BytesRemoved)
	}
}

func TestCompare_MovieIdentity_FallsBackToTitleYearWhenTMDBIDZero(t *testing.T) {
	prev := snap([]inventory.Movie{movie(0, "No TMDB Match", 2019, 100)}, nil)
	cur := snap([]inventory.Movie{movie(0, "No TMDB Match", 2019, 100)}, nil)

	d := Compare(prev, true, cur)

	if d.MoviesAdded != 0 || d.MoviesRemoved != 0 {
		t.Errorf("MoviesAdded/Removed = %d/%d, want 0/0 - title+year fallback should match", d.MoviesAdded, d.MoviesRemoved)
	}
}

func TestCompare_SeriesIdentity_FallsBackToTitleYearWhenTVDBIDZero(t *testing.T) {
	prev := snap(nil, []inventory.Series{seriesWith(0, "No TVDB Match", 2019, ep(1, 1, 10))})
	cur := snap(nil, []inventory.Series{seriesWith(0, "No TVDB Match", 2019, ep(1, 1, 10))})

	d := Compare(prev, true, cur)

	if d.SeriesAdded != 0 || d.SeriesRemoved != 0 {
		t.Errorf("SeriesAdded/Removed = %d/%d, want 0/0 - title+year fallback should match", d.SeriesAdded, d.SeriesRemoved)
	}
}

func TestCompare_PercentChange_ShrinkingLibraryIsNegative(t *testing.T) {
	prev := snap([]inventory.Movie{movie(1, "A", 2000, 100), movie(2, "B", 2001, 100)}, nil)
	cur := snap([]inventory.Movie{movie(1, "A", 2000, 100)}, nil)

	d := Compare(prev, true, cur)

	if d.TotalSizeChangePercent >= 0 {
		t.Errorf("TotalSizeChangePercent = %v, want negative for a shrinking library", d.TotalSizeChangePercent)
	}
	if d.TotalSizeChangePercent != -50 {
		t.Errorf("TotalSizeChangePercent = %v, want -50", d.TotalSizeChangePercent)
	}
}

func TestCompare_PrevTotalBytesZero_PercentIsZeroNotInfOrNaN(t *testing.T) {
	prev := snap(nil, nil)
	cur := snap([]inventory.Movie{movie(1, "A", 2000, 100)}, nil)

	d := Compare(prev, true, cur)

	if math.IsInf(d.TotalSizeChangePercent, 0) || math.IsNaN(d.TotalSizeChangePercent) {
		t.Fatalf("TotalSizeChangePercent = %v, want a finite number", d.TotalSizeChangePercent)
	}
	if d.TotalSizeChangePercent != 0 {
		t.Errorf("TotalSizeChangePercent = %v, want 0 when prev had zero bytes", d.TotalSizeChangePercent)
	}
}

func TestCompare_FirstRun_AllDeltasZero(t *testing.T) {
	cur := snap([]inventory.Movie{movie(1, "A", 2000, 100)}, []inventory.Series{seriesWith(1, "S", 2000, ep(1, 1, 50))})

	d := Compare(inventory.Snapshot{}, false, cur)

	if !d.IsFirstRun {
		t.Error("IsFirstRun = false, want true")
	}
	if d.MoviesAdded != 0 || d.EpisodesAdded != 0 || d.BytesAdded != 0 {
		t.Errorf("d = %+v, want all deltas zero on first run even though cur has content", d)
	}
}

func TestCompare_ReorderedSlices_ProduceIdenticalDiff(t *testing.T) {
	prev := snap(
		[]inventory.Movie{movie(1, "A", 2000, 100), movie(2, "B", 2001, 200)},
		nil,
	)
	curInOrder := snap(
		[]inventory.Movie{movie(1, "A", 2000, 100), movie(2, "B", 2001, 200), movie(3, "C", 2002, 300)},
		nil,
	)
	curReordered := snap(
		[]inventory.Movie{movie(3, "C", 2002, 300), movie(2, "B", 2001, 200), movie(1, "A", 2000, 100)},
		nil,
	)

	d1 := Compare(prev, true, curInOrder)
	d2 := Compare(prev, true, curReordered)

	if d1 != d2 {
		t.Errorf("diff depends on slice order: %+v vs %+v", d1, d2)
	}
}
