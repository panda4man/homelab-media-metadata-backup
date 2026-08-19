# media-inventory

A disaster-recovery inventory for an Unraid media library. Once a week it
walks the movie and TV filesystems, matches what it finds against Radarr
and Sonarr metadata, and writes a versioned JSON manifest — plus CSV
exports for humans and a SHA-256 checksum for integrity — to a local
snapshot directory, then syncs that directory off-site via `rclone`.

The manifest records **what media physically existed on disk**, not what
Radarr/Sonarr expect to exist. If the entire Unraid array is lost, the
off-site JSON is enough to know exactly which movies and episodes to
re-acquire, with their canonical TMDB/TVDB IDs.

## Success criterion

An administrator who loses the entire Unraid media filesystem can retrieve
the off-site `last-known-good.json` and determine:

- Every movie that existed, with its canonical TMDB ID.
- Every TV series that existed, with its canonical TVDB ID.
- Exactly which episodes existed.
- The approximate size and original relative path of each file.

Normal operation also publishes metrics to InfluxDB so Grafana can answer:
*does this week's inventory look reasonable compared with previous weeks?*
A sudden drop in files, episodes, movies, or total media bytes is visible
and alertable — before a bad inventory quietly replaces the known-good
disaster-recovery record.

## How it works

Each scheduled run: verifies Sonarr and Radarr are reachable, loads their
metadata, walks the configured media roots, matches files against that
metadata (a Radarr/Sonarr entry with no file on disk is never reported;
the filesystem is the sole authority on what exists), builds a snapshot,
diffs it against the previous one, runs sanity checks, writes the dated
JSON/CSV/SHA-256 files, updates `latest.json` and (if valid)
`last-known-good.json`, syncs off-site, publishes metrics, and prunes old
snapshots — all behind a single-instance lock so two runs can never
overlap.

### Snapshot states

Every run resolves to exactly one of three states:

| State     | Meaning                                                          | `latest.json` | `last-known-good.json` |
|-----------|-------------------------------------------------------------------|:---:|:---:|
| `valid`   | Scan completed, no sanity checks triggered.                       | ✅ | ✅ |
| `warning` | Scan completed, but a significant change was detected (still retained for forensics). | ✅ | ❌ |
| `failed`  | Radarr/Sonarr unreachable, a media root inaccessible/unexpectedly empty, or the scan was cancelled before finishing. | ❌ | ❌ |

The dated snapshot file itself is written and kept in **all three**
states — even a failed run's snapshot has forensic value. Only the
`latest.json`/`last-known-good.json` pointers are conditional.

Off-site sync success/failure is tracked **separately** from snapshot
state: a `valid` snapshot whose off-site copy failed to upload is not
"successful" for disaster-recovery purposes (see exit codes below), but
it does not retroactively make the snapshot itself untrustworthy.

## Setup

1. Copy `.env.example` to `.env` and fill in:
   - `RADARR_URL`/`RADARR_API_KEY`, `SONARR_URL`/`SONARR_API_KEY`.
   - `HOST_MEDIA_MOVIES_PATH`, `HOST_MEDIA_TV_PATH` — the real host paths
     to your movies and TV libraries (mounted read-only).
   - `HOST_SNAPSHOT_PATH` — where snapshots live on the host (mounted
     read-write; this is what gets synced off-site).
   - `RCLONE_REMOTE` — an rclone remote name already configured in your
     `rclone.conf` (e.g. `media-inventory:` for Backblaze B2, S3, etc.).
     Leave empty to disable off-site sync entirely.
   - `INFLUX_URL`/`INFLUX_TOKEN`/`INFLUX_ORG`/`INFLUX_BUCKET` if you want
     metrics in Grafana. Leave `INFLUX_URL` empty to disable — InfluxDB is
     observability only and is never required for a valid snapshot.

   The container runs as a fixed non-root UID/GID `1000:1000`. Make sure
   `HOST_SNAPSHOT_PATH` is writable by that UID on the host, e.g.
   `chown -R 1000:1000 /mnt/user/appdata/media-inventory`.

   Leave `MEDIA_MOVIES_PATH`, `MEDIA_TV_PATH`, and `SNAPSHOT_PATH` as-is —
   they're the container-internal paths the application reads, and must
   match the container side of the volume mounts in `compose.yml`.

2. rclone needs its own config file with your remote's credentials,
   mounted into the container (not shown in the default `compose.yml`,
   since remotes vary widely — add a volume mount for
   `~/.config/rclone/rclone.conf:/app/.config/rclone/rclone.conf:ro` if
   you use off-site sync).

3. Build and start:

   ```bash
   docker compose up -d --build
   ```

   The container stays running under `supercronic`, which fires the job
   every Sunday at 23:00 in the configured `TZ`. Nothing runs immediately
   on startup — see below to trigger a run manually.

4. Run manually (e.g. to test before waiting for Sunday):

   ```bash
   docker compose run --rm media-inventory /app/media-inventory run
   ```

## Exit codes

| Code | Meaning |
|:---:|---|
| 0 | Success — snapshot is `valid` or `warning`, off-site sync succeeded or is disabled. |
| 1 | Failed run — snapshot state is `failed`. |
| 2 | Usage or configuration error (missing/invalid env vars). |
| 3 | Another run is already in progress. |
| 4 | Off-site backup failure — the snapshot itself is valid/warning, but the off-site copy failed. |

## Disaster recovery

If the Unraid array is lost, pull `last-known-good.json` from your
off-site remote (e.g. `rclone copy media-inventory:snapshots/last-known-good.json .`).
It contains, for every movie and TV series that existed at that snapshot:

```json
{
  "schema_version": 1,
  "generated_at": "2026-08-16T23:00:00Z",
  "hostname": "unraid",
  "summary": { "movies": 4284, "series": 317, "episodes": 21506, "media_files": 25790, "total_bytes": 42648192382976 },
  "movies": [
    { "title": "Inception", "year": 2010, "tmdb_id": 27205, "imdb_id": "tt1375666", "dir": "Inception (2010)", "path": "Inception (2010)/Inception.mkv", "bytes": 3481932841, "mtime": "..." }
  ],
  "series": [
    { "title": "Severance", "year": 2022, "tvdb_id": 371980, "dir": "Severance (2022)", "episodes": [
      { "season": 1, "episode": 1, "title": "Good News About Hell", "path": "Severance (2022)/Season 01/Severance - S01E01.mkv", "bytes": 5000000000, "mtime": "..." }
    ]}
  ]
}
```

`tmdb_id` and `tvdb_id` are the canonical identifiers — feed them into a
fresh Radarr/Sonarr instance to re-acquire each title. `path` and `bytes`
tell you the original relative location and approximate size. Verify the
file wasn't corrupted in transit with the matching `.sha256` sidecar:

```bash
sha256sum -c last-known-good.json.sha256
```

The `-movies.csv`/`-episodes.csv` exports next to each dated JSON snapshot
are a convenience for manually skimming the inventory — JSON remains the
authoritative record.

## Threshold tuning

The default sanity-check thresholds (all in `.env.example`) are
intentionally conservative:

| Variable | Default | Triggers when... |
|---|:---:|---|
| `MAX_MEDIA_BYTES_DECREASE_PERCENT` | 5 | total media size drops more than this |
| `MAX_MOVIE_DECREASE_PERCENT` | 5 | movie count drops more than this |
| `MAX_EPISODE_DECREASE_PERCENT` | 5 | episode count drops more than this |
| `MAX_FILES_REMOVED` | 100 | more than this many files disappear in one run |
| `MAX_UNMATCHED_PERCENT` | 5 | more than this % of discovered files can't be matched to Radarr/Sonarr |

These five are "warning" level — the run still completes and both the
dated snapshot and `latest.json` are written, but `last-known-good.json`
is not updated. Radarr/Sonarr unreachable, a media root inaccessible or
unexpectedly empty, and a scan that terminates early are always "failed"
regardless of threshold — those represent the run being unable to
confidently determine the library's actual state at all.

If your library genuinely churns a lot (e.g. large seasonal deletions),
raise the relevant threshold rather than disabling the check.

## Metrics reference

When `INFLUX_URL` is set, every run (including failed ones) publishes one
`media_inventory` point per run to InfluxDB v2, tagged with
`host`, `job=media-inventory`, `schema_version=1` — deliberately no
per-title or per-ID tags, to keep cardinality bounded. Fields:

`movies`, `series`, `episodes`, `media_files`, `total_bytes`,
`movies_added`, `movies_removed`, `series_added`, `series_removed`,
`episodes_added`, `episodes_removed`, `files_added`, `files_removed`,
`bytes_added`, `bytes_removed`, `unmatched_files`,
`scan_duration_seconds`, `snapshot_valid`, `snapshot_warning`,
`offsite_upload_success`.

Booleans (`snapshot_valid`, `snapshot_warning`, `offsite_upload_success`)
are encoded as `1i`/`0i` rather than native line-protocol booleans, so
Grafana can sum/average them directly (e.g. "successful runs this month")
without a cast.

A useful Grafana panel set: current inventory (movies/series/episodes/
size), historical growth over time, weekly added/removed deltas, and an
inventory-health panel showing last successful run, last successful
off-site upload, scan duration, and unmatched-file count.

## Retention

Five weekly snapshots are kept by default (`SNAPSHOT_RETENTION=5`), plus
`latest.json` and `last-known-good.json`, which don't count toward the
limit. The dated snapshot `last-known-good.json` currently points to is
never pruned, even if it would otherwise fall outside the retention
window — the disaster-recovery anchor can't be pruned out from under
itself.

## Development

```bash
make test    # go test -race ./... - no network or Docker required
make lint    # gofmt -l . && go vet ./...
make build   # local binary at bin/media-inventory
```

Integration tests that talk to a real Radarr/Sonarr/InfluxDB/rclone are
opt-in via the `integration` build tag and are not part of the default
test run.
