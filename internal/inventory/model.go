// Package inventory holds the domain model for a media-inventory snapshot:
// the authoritative, versioned record of every movie and TV episode that
// actually existed on disk at scan time. This package depends on nothing
// else in the module so the schema stays a stable, standalone contract.
package inventory

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const schemaVersion = 1

// ErrUnsupportedSchema is returned when unmarshaling a snapshot whose
// schema_version this build does not know how to read.
var ErrUnsupportedSchema = errors.New("unsupported snapshot schema version")

// Snapshot is the authoritative disaster-recovery artifact: everything
// needed to reconstruct the media library from an off-site copy.
type Snapshot struct {
	SchemaVersion int       `json:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at"`
	Hostname      string    `json:"hostname"`
	Summary       Summary   `json:"summary"`
	Movies        []Movie   `json:"movies"`
	Series        []Series  `json:"series"`
}

// Summary holds the aggregate counters shown at the top of a snapshot.
type Summary struct {
	Movies     int   `json:"movies"`
	Series     int   `json:"series"`
	Episodes   int   `json:"episodes"`
	MediaFiles int   `json:"media_files"`
	TotalBytes int64 `json:"total_bytes"`
}

// Recompute derives every counter from the given movies and series,
// overwriting whatever the Summary previously held.
func (s *Summary) Recompute(movies []Movie, series []Series) {
	episodes := 0
	var totalBytes int64
	for _, m := range movies {
		totalBytes += m.Bytes
	}
	for _, sr := range series {
		episodes += len(sr.Episodes)
		for _, e := range sr.Episodes {
			totalBytes += e.Bytes
		}
	}
	s.Movies = len(movies)
	s.Series = len(series)
	s.Episodes = episodes
	s.MediaFiles = len(movies) + episodes
	s.TotalBytes = totalBytes
}

// Movie is a single movie file matched against Radarr metadata. TMDBID is
// the canonical recovery identifier.
type Movie struct {
	Title  string    `json:"title"`
	Year   int       `json:"year"`
	TMDBID int       `json:"tmdb_id"`
	IMDbID string    `json:"imdb_id,omitempty"`
	Dir    string    `json:"dir"`
	Path   string    `json:"path"`
	Bytes  int64     `json:"bytes"`
	MTime  time.Time `json:"mtime"`
}

// Series is a TV series with at least one episode file matched against
// Sonarr metadata. TVDBID is the canonical recovery identifier.
type Series struct {
	Title    string    `json:"title"`
	Year     int       `json:"year"`
	TVDBID   int       `json:"tvdb_id"`
	TMDBID   int       `json:"tmdb_id,omitempty"`
	Dir      string    `json:"dir"`
	Episodes []Episode `json:"episodes"`
}

// Episode is a single episode file that exists on disk.
type Episode struct {
	Season  int       `json:"season"`
	Episode int       `json:"episode"`
	Title   string    `json:"title"`
	Path    string    `json:"path"`
	Bytes   int64     `json:"bytes"`
	MTime   time.Time `json:"mtime"`
}

// NewSnapshot builds a Snapshot with SchemaVersion always set to the
// current version and Summary derived from the given movies and series.
func NewSnapshot(generatedAt time.Time, hostname string, movies []Movie, series []Series) Snapshot {
	s := Snapshot{
		SchemaVersion: schemaVersion,
		GeneratedAt:   generatedAt,
		Hostname:      hostname,
		Movies:        movies,
		Series:        series,
	}
	s.Summary.Recompute(movies, series)
	return s
}

// MarshalJSON forces GeneratedAt to serialize as UTC RFC3339 regardless of
// the time.Time's original location, so snapshots are comparable byte-for-
// byte no matter what timezone the container ran in.
func (s Snapshot) MarshalJSON() ([]byte, error) {
	type alias Snapshot
	return json.Marshal(struct {
		GeneratedAt string `json:"generated_at"`
		alias
	}{
		GeneratedAt: s.GeneratedAt.UTC().Format(time.RFC3339),
		alias:       alias(s),
	})
}

// UnmarshalJSON rejects any schema_version this build does not understand,
// so a future incompatible format fails loudly instead of decoding into a
// silently wrong struct.
func (s *Snapshot) UnmarshalJSON(data []byte) error {
	type alias Snapshot
	aux := struct{ *alias }{alias: (*alias)(s)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if s.SchemaVersion != schemaVersion {
		return fmt.Errorf("%w: got %d, want %d", ErrUnsupportedSchema, s.SchemaVersion, schemaVersion)
	}
	return nil
}
