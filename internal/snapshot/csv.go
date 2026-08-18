package snapshot

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/inventory"
)

func moviesCSVFilename(now time.Time) string {
	return fmt.Sprintf("media-inventory-%s-movies.csv", now.Format("2006-01-02"))
}

func episodesCSVFilename(now time.Time) string {
	return fmt.Sprintf("media-inventory-%s-episodes.csv", now.Format("2006-01-02"))
}

// WriteMoviesCSV writes the movies convenience export atomically. JSON
// remains the authoritative snapshot; this is for manual inspection.
func WriteMoviesCSV(dir string, movies []inventory.Movie, now time.Time) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if err := w.Write([]string{"title", "year", "tmdb_id", "imdb_id", "path", "bytes"}); err != nil {
		return "", fmt.Errorf("snapshot: writing movies csv header: %w", err)
	}
	for _, m := range movies {
		row := []string{
			m.Title,
			strconv.Itoa(m.Year),
			strconv.Itoa(m.TMDBID),
			m.IMDbID,
			m.Path,
			strconv.FormatInt(m.Bytes, 10),
		}
		if err := w.Write(row); err != nil {
			return "", fmt.Errorf("snapshot: writing movies csv row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", fmt.Errorf("snapshot: flushing movies csv: %w", err)
	}

	return writeAtomic(dir, moviesCSVFilename(now), buf.Bytes(), nil)
}

// WriteEpisodesCSV writes the episodes convenience export atomically.
func WriteEpisodesCSV(dir string, series []inventory.Series, now time.Time) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if err := w.Write([]string{"series_title", "year", "tvdb_id", "season", "episode", "episode_title", "path", "bytes"}); err != nil {
		return "", fmt.Errorf("snapshot: writing episodes csv header: %w", err)
	}
	for _, s := range series {
		for _, e := range s.Episodes {
			row := []string{
				s.Title,
				strconv.Itoa(s.Year),
				strconv.Itoa(s.TVDBID),
				strconv.Itoa(e.Season),
				strconv.Itoa(e.Episode),
				e.Title,
				e.Path,
				strconv.FormatInt(e.Bytes, 10),
			}
			if err := w.Write(row); err != nil {
				return "", fmt.Errorf("snapshot: writing episodes csv row: %w", err)
			}
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", fmt.Errorf("snapshot: flushing episodes csv: %w", err)
	}

	return writeAtomic(dir, episodesCSVFilename(now), buf.Bytes(), nil)
}
