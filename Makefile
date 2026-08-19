.PHONY: build test lint docker-build integration

build:
	go build -o bin/media-inventory ./cmd/media-inventory

test:
	go test -race ./...

lint:
	gofmt -l .
	go vet ./...

docker-build:
	docker build -t media-inventory .

# Opt-in integration tests against real Radarr/Sonarr/InfluxDB. Requires
# MEDIA_INVENTORY_IT=1 plus the relevant *_URL/*_API_KEY/*_TOKEN env vars;
# each test skips on its own if its prerequisites aren't set.
integration:
	MEDIA_INVENTORY_IT=1 go test -tags=integration ./...
