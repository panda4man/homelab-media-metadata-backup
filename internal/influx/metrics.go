package influx

// Metrics is one run's worth of aggregate operational data, published as
// the "media_inventory" measurement. Fields only - never titles or IDs,
// which would create unbounded tag cardinality.
type Metrics struct {
	Movies     int
	Series     int
	Episodes   int
	MediaFiles int
	TotalBytes int64

	MoviesAdded, MoviesRemoved     int
	SeriesAdded, SeriesRemoved     int
	EpisodesAdded, EpisodesRemoved int
	FilesAdded, FilesRemoved       int
	BytesAdded, BytesRemoved       int64

	UnmatchedFiles      int
	ScanDurationSeconds float64

	SnapshotValid        bool
	SnapshotWarning      bool
	OffsiteUploadSuccess bool
}

// Tags are the only InfluxDB tags this measurement uses. Deliberately
// low-cardinality - no per-title or per-ID tags.
type Tags struct {
	Host          string
	Job           string
	SchemaVersion int
}
