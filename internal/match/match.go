// Package match joins the filesystem's view of what actually exists
// against Radarr/Sonarr metadata to build the matched, disaster-recovery
// inventory. The filesystem is the sole authority on existence: a movie or
// episode with no corresponding file on disk is never reported, no matter
// what Radarr or Sonarr believe.
package match

import (
	"path"
	"sort"

	"github.com/panda4man/homelab-media-metadata-backup/internal/filesystem"
	"github.com/panda4man/homelab-media-metadata-backup/internal/inventory"
	"github.com/panda4man/homelab-media-metadata-backup/internal/radarr"
	"github.com/panda4man/homelab-media-metadata-backup/internal/sonarr"
)

// MatchMovies joins walked movie files against Radarr's movie list.
// files should already be scoped to the movies root. A Radarr movie with
// no matching file (whether because Radarr has no file on disk, or its
// expected file simply is not present) is silently excluded - it is not
// reported as existing. Files with no matching Radarr movie are returned
// as unmatched.
func MatchMovies(files []filesystem.File, movies []radarr.Movie) (matched []inventory.Movie, unmatched []filesystem.File) {
	byExact, byFold := indexFiles(files)
	used := make(map[int]bool, len(files))

	for _, m := range movies {
		if m.RelativePath == "" {
			continue // Radarr has no file for this movie; nothing to look up.
		}
		dir := path.Base(m.Path)
		expected := path.Join(dir, m.RelativePath)

		idx, ok := lookupFile(expected, byExact, byFold)
		if !ok || used[idx] {
			continue
		}
		used[idx] = true
		f := files[idx]
		matched = append(matched, inventory.Movie{
			Title:  m.Title,
			Year:   m.Year,
			TMDBID: m.TMDBID,
			IMDbID: m.IMDbID,
			Dir:    dir,
			Path:   f.RelPath,
			Bytes:  f.Bytes,
			MTime:  f.ModTime,
		})
	}

	unmatched = collectUnused(files, used)

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].Title != matched[j].Title {
			return matched[i].Title < matched[j].Title
		}
		return matched[i].Year < matched[j].Year
	})
	return matched, unmatched
}

// SeriesWithEpisodes pairs a Sonarr series with the episodes Sonarr
// reported for it (already scoped by series ID at the API layer).
type SeriesWithEpisodes struct {
	Series   sonarr.Series
	Episodes []sonarr.Episode
}

// MatchEpisodes joins walked TV files against Sonarr's series/episode
// metadata. files should already be scoped to the TV root. A series ends
// up in the result only if at least one of its episodes actually has a
// file on disk; a series with zero matched episodes is excluded entirely.
func MatchEpisodes(files []filesystem.File, seriesList []SeriesWithEpisodes) (matched []inventory.Series, unmatched []filesystem.File) {
	byExact, byFold := indexFiles(files)
	used := make(map[int]bool, len(files))

	for _, sw := range seriesList {
		dir := path.Base(sw.Series.Path)
		var episodes []inventory.Episode

		for _, e := range sw.Episodes {
			if e.RelativePath == "" {
				continue // Sonarr has no file for this episode.
			}
			expected := path.Join(dir, e.RelativePath)

			idx, ok := lookupFile(expected, byExact, byFold)
			if !ok {
				continue
			}
			// Unlike movies, a single file may legitimately back more than
			// one episode (a multi-episode file) - do not treat a repeat
			// claim on the same file as a conflict.
			used[idx] = true
			f := files[idx]
			episodes = append(episodes, inventory.Episode{
				Season:  e.Season,
				Episode: e.Episode,
				Title:   e.Title,
				Path:    f.RelPath,
				Bytes:   f.Bytes,
				MTime:   f.ModTime,
			})
		}

		if len(episodes) == 0 {
			continue // A series only "exists" if at least one episode file does.
		}

		sort.Slice(episodes, func(i, j int) bool {
			if episodes[i].Season != episodes[j].Season {
				return episodes[i].Season < episodes[j].Season
			}
			return episodes[i].Episode < episodes[j].Episode
		})

		matched = append(matched, inventory.Series{
			Title:    sw.Series.Title,
			Year:     sw.Series.Year,
			TVDBID:   sw.Series.TVDBID,
			TMDBID:   sw.Series.TMDBID,
			Dir:      dir,
			Episodes: episodes,
		})
	}

	unmatched = collectUnused(files, used)

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].Title != matched[j].Title {
			return matched[i].Title < matched[j].Title
		}
		return matched[i].Year < matched[j].Year
	})
	return matched, unmatched
}

// UnmatchedPercent returns what fraction of totalFiles were unmatched, as
// a percentage. It never divides by zero: with no files at all, the
// answer is 0, not NaN or Inf.
func UnmatchedPercent(unmatchedCount, totalFiles int) float64 {
	if totalFiles == 0 {
		return 0
	}
	return float64(unmatchedCount) / float64(totalFiles) * 100
}

// indexFiles builds an exact-case lookup and a case-folded fallback
// lookup from a file's normalized relative path to its index in files.
func indexFiles(files []filesystem.File) (byExact, byFold map[string]int) {
	byExact = make(map[string]int, len(files))
	byFold = make(map[string]int, len(files))
	for i, f := range files {
		byExact[normalizeRelPath(f.RelPath)] = i
		byFold[normalizeRelPathFold(f.RelPath)] = i
	}
	return byExact, byFold
}

// lookupFile finds a file index for an expected relative path, trying an
// exact-case match before falling back to a case-insensitive one.
func lookupFile(expected string, byExact, byFold map[string]int) (int, bool) {
	if idx, ok := byExact[normalizeRelPath(expected)]; ok {
		return idx, true
	}
	idx, ok := byFold[normalizeRelPathFold(expected)]
	return idx, ok
}

func collectUnused(files []filesystem.File, used map[int]bool) []filesystem.File {
	var unmatched []filesystem.File
	for i, f := range files {
		if !used[i] {
			unmatched = append(unmatched, f)
		}
	}
	sort.Slice(unmatched, func(i, j int) bool {
		return unmatched[i].RelPath < unmatched[j].RelPath
	})
	return unmatched
}
