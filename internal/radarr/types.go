package radarr

// Movie is the lean, mapped view of a Radarr movie: only the fields the
// disaster-recovery manifest needs, not the full Radarr API payload.
type Movie struct {
	Title        string
	Year         int
	TMDBID       int
	IMDbID       string
	Path         string
	RelativePath string
	Bytes        int64
	HasFile      bool
}

// movieDTO mirrors the fields Radarr's GET /api/v3/movie response actually
// carries, unmapped.
type movieDTO struct {
	Title     string        `json:"title"`
	Year      int           `json:"year"`
	TmdbID    int           `json:"tmdbId"`
	ImdbID    string        `json:"imdbId"`
	Path      string        `json:"path"`
	HasFile   bool          `json:"hasFile"`
	MovieFile *movieFileDTO `json:"movieFile"`
}

type movieFileDTO struct {
	RelativePath string `json:"relativePath"`
	Size         int64  `json:"size"`
}

func (d movieDTO) toMovie() Movie {
	m := Movie{
		Title:   d.Title,
		Year:    d.Year,
		TMDBID:  d.TmdbID,
		IMDbID:  d.ImdbID,
		Path:    d.Path,
		HasFile: d.HasFile,
	}
	if d.MovieFile != nil {
		m.RelativePath = d.MovieFile.RelativePath
		m.Bytes = d.MovieFile.Size
	}
	return m
}
