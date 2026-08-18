package sonarr

// Series is the lean, mapped view of a Sonarr series: only the fields the
// disaster-recovery manifest needs.
type Series struct {
	ID     int
	Title  string
	Year   int
	TVDBID int
	TMDBID int
	Path   string
}

// Episode is a single episode, joined with its file if one exists.
type Episode struct {
	Season       int
	Episode      int
	Title        string
	HasFile      bool
	RelativePath string
	Bytes        int64
}

type seriesDTO struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Year   int    `json:"year"`
	TvdbID int    `json:"tvdbId"`
	TmdbID int    `json:"tmdbId"`
	Path   string `json:"path"`
}

func (d seriesDTO) toSeries() Series {
	return Series{
		ID:     d.ID,
		Title:  d.Title,
		Year:   d.Year,
		TVDBID: d.TvdbID,
		TMDBID: d.TmdbID,
		Path:   d.Path,
	}
}

type episodeDTO struct {
	ID            int    `json:"id"`
	SeriesID      int    `json:"seriesId"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	Title         string `json:"title"`
	HasFile       bool   `json:"hasFile"`
	EpisodeFileID int    `json:"episodeFileId"`
}

type episodeFileDTO struct {
	ID           int    `json:"id"`
	SeriesID     int    `json:"seriesId"`
	RelativePath string `json:"relativePath"`
	Size         int64  `json:"size"`
}

// joinEpisodes maps episode metadata to its file by episodeFileId. A file
// may be shared by more than one episode entry (multi-episode files).
func joinEpisodes(episodes []episodeDTO, files []episodeFileDTO) []Episode {
	fileByID := make(map[int]episodeFileDTO, len(files))
	for _, f := range files {
		fileByID[f.ID] = f
	}

	result := make([]Episode, 0, len(episodes))
	for _, e := range episodes {
		ep := Episode{
			Season:  e.SeasonNumber,
			Episode: e.EpisodeNumber,
			Title:   e.Title,
			HasFile: e.HasFile,
		}
		if e.HasFile {
			if f, ok := fileByID[e.EpisodeFileID]; ok {
				ep.RelativePath = f.RelativePath
				ep.Bytes = f.Size
			}
		}
		result = append(result, ep)
	}
	return result
}
