// Package validation compares snapshots and evaluates sanity checks
// against the result.
package validation

import (
	"fmt"

	"github.com/panda4man/homelab-media-metadata-backup/internal/inventory"
)

// Diff is the delta between the previous successful snapshot and the
// current one. Percentages are computed as (cur-prev)/prev*100, so a
// shrinking library produces a negative percentage; when prev is zero the
// percentage is 0, never NaN or +Inf.
type Diff struct {
	IsFirstRun bool

	MoviesAdded, MoviesRemoved     int
	SeriesAdded, SeriesRemoved     int
	EpisodesAdded, EpisodesRemoved int
	FilesAdded, FilesRemoved       int
	BytesAdded, BytesRemoved       int64

	TotalSizeChangePercent float64
	MovieChangePercent     float64
	EpisodeChangePercent   float64
}

// Compare computes the Diff between prev and cur. prevFound mirrors
// snapshot.LoadPrevious's second return value: when false (no previous
// snapshot exists yet), Compare returns a zero Diff with IsFirstRun set,
// regardless of what prev/cur contain - a first run is never mistaken for
// catastrophic loss.
func Compare(prev inventory.Snapshot, prevFound bool, cur inventory.Snapshot) Diff {
	if !prevFound {
		return Diff{IsFirstRun: true}
	}

	var d Diff
	diffMovies(&d, prev.Movies, cur.Movies)
	diffSeries(&d, prev.Series, cur.Series)
	d.FilesAdded = d.MoviesAdded + d.EpisodesAdded
	d.FilesRemoved = d.MoviesRemoved + d.EpisodesRemoved

	d.TotalSizeChangePercent = percentChange(float64(prev.Summary.TotalBytes), float64(cur.Summary.TotalBytes))
	d.MovieChangePercent = percentChange(float64(prev.Summary.Movies), float64(cur.Summary.Movies))
	d.EpisodeChangePercent = percentChange(float64(prev.Summary.Episodes), float64(cur.Summary.Episodes))
	return d
}

func percentChange(prevVal, curVal float64) float64 {
	if prevVal == 0 {
		return 0
	}
	return (curVal - prevVal) / prevVal * 100
}

func movieKey(m inventory.Movie) string {
	if m.TMDBID != 0 {
		return fmt.Sprintf("tmdb:%d", m.TMDBID)
	}
	return fmt.Sprintf("title:%s|%d", m.Title, m.Year)
}

func diffMovies(d *Diff, prev, cur []inventory.Movie) {
	prevByKey := make(map[string]inventory.Movie, len(prev))
	for _, m := range prev {
		prevByKey[movieKey(m)] = m
	}
	curByKey := make(map[string]inventory.Movie, len(cur))
	for _, m := range cur {
		curByKey[movieKey(m)] = m
	}

	for key, c := range curByKey {
		p, inPrev := prevByKey[key]
		if !inPrev {
			d.MoviesAdded++
			d.BytesAdded += c.Bytes
			continue
		}
		addByteDelta(d, p.Bytes, c.Bytes)
	}
	for key, p := range prevByKey {
		if _, inCur := curByKey[key]; !inCur {
			d.MoviesRemoved++
			d.BytesRemoved += p.Bytes
		}
	}
}

func seriesKey(s inventory.Series) string {
	if s.TVDBID != 0 {
		return fmt.Sprintf("tvdb:%d", s.TVDBID)
	}
	return fmt.Sprintf("title:%s|%d", s.Title, s.Year)
}

func episodeKey(e inventory.Episode) string {
	return fmt.Sprintf("S%02dE%02d", e.Season, e.Episode)
}

func diffSeries(d *Diff, prev, cur []inventory.Series) {
	prevByKey := make(map[string]inventory.Series, len(prev))
	for _, s := range prev {
		prevByKey[seriesKey(s)] = s
	}
	curByKey := make(map[string]inventory.Series, len(cur))
	for _, s := range cur {
		curByKey[seriesKey(s)] = s
	}

	for key, c := range curByKey {
		p, inPrev := prevByKey[key]
		if !inPrev {
			d.SeriesAdded++
			d.EpisodesAdded += len(c.Episodes)
			for _, e := range c.Episodes {
				d.BytesAdded += e.Bytes
			}
			continue
		}
		diffEpisodes(d, p.Episodes, c.Episodes)
	}
	for key, p := range prevByKey {
		if _, inCur := curByKey[key]; !inCur {
			d.SeriesRemoved++
			d.EpisodesRemoved += len(p.Episodes)
			for _, e := range p.Episodes {
				d.BytesRemoved += e.Bytes
			}
		}
	}
}

func diffEpisodes(d *Diff, prev, cur []inventory.Episode) {
	prevByKey := make(map[string]inventory.Episode, len(prev))
	for _, e := range prev {
		prevByKey[episodeKey(e)] = e
	}
	curByKey := make(map[string]inventory.Episode, len(cur))
	for _, e := range cur {
		curByKey[episodeKey(e)] = e
	}

	for key, c := range curByKey {
		p, inPrev := prevByKey[key]
		if !inPrev {
			d.EpisodesAdded++
			d.BytesAdded += c.Bytes
			continue
		}
		addByteDelta(d, p.Bytes, c.Bytes)
	}
	for key, p := range prevByKey {
		if _, inCur := curByKey[key]; !inCur {
			d.EpisodesRemoved++
			d.BytesRemoved += p.Bytes
		}
	}
}

// addByteDelta credits a size change between two versions of the same
// identity (same TMDB/TVDB ID, different file) to BytesAdded or
// BytesRemoved without touching the added/removed counts.
func addByteDelta(d *Diff, prevBytes, curBytes int64) {
	delta := curBytes - prevBytes
	if delta > 0 {
		d.BytesAdded += delta
	} else if delta < 0 {
		d.BytesRemoved += -delta
	}
}
