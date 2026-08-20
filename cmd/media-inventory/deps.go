package main

import (
	"log/slog"

	"github.com/panda4man/homelab-media-metadata-backup/internal/clockx"
	"github.com/panda4man/homelab-media-metadata-backup/internal/config"
	"github.com/panda4man/homelab-media-metadata-backup/internal/filesystem"
	"github.com/panda4man/homelab-media-metadata-backup/internal/influx"
	"github.com/panda4man/homelab-media-metadata-backup/internal/offsite"
	"github.com/panda4man/homelab-media-metadata-backup/internal/orchestrator"
	"github.com/panda4man/homelab-media-metadata-backup/internal/radarr"
	"github.com/panda4man/homelab-media-metadata-backup/internal/snapshot"
	"github.com/panda4man/homelab-media-metadata-backup/internal/sonarr"
)

// buildDeps wires every package into a real orchestrator.Deps for cfg,
// logging through logger and tagging published metrics with hostname.
func buildDeps(cfg config.Config, logger *slog.Logger, hostname string) orchestrator.Deps {
	radarrClient := radarr.New(cfg.RadarrURL, cfg.RadarrAPIKey)
	sonarrClient := sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey)
	offsiteSyncer := offsite.Syncer{Runner: offsite.ExecRunner{}, Remote: cfg.RcloneRemote, Logger: logger}

	publisher := influx.Publisher{
		Tags:   influx.Tags{Host: hostname, Job: "media-inventory", SchemaVersion: 1},
		Clock:  clockx.System{},
		Logger: logger,
	}
	if cfg.InfluxURL != "" {
		publisher.Client = influx.New(cfg.InfluxURL, cfg.InfluxToken, cfg.InfluxOrg, cfg.InfluxBucket)
	}

	return orchestrator.Deps{
		Clock:    clockx.System{},
		Logger:   logger,
		Hostname: hostname,

		WalkRoots: filesystem.WalkRoots,

		RadarrPing:   radarrClient.Ping,
		RadarrMovies: radarrClient.Movies,

		SonarrPing:     sonarrClient.Ping,
		SonarrSeries:   sonarrClient.Series,
		SonarrEpisodes: sonarrClient.Episodes,

		WriteJSON:           snapshot.WriteJSON,
		WriteSHA256:         snapshot.WriteSHA256,
		WriteMoviesCSV:      snapshot.WriteMoviesCSV,
		WriteEpisodesCSV:    snapshot.WriteEpisodesCSV,
		LoadPrevious:        snapshot.LoadPrevious,
		UpdateLatest:        snapshot.UpdateLatest,
		UpdateLastKnownGood: snapshot.UpdateLastKnownGood,
		Prune:               snapshot.Prune,

		OffsiteSync: offsiteSyncer.Sync,

		PublishMetrics: publisher.Publish,
	}
}
