.PHONY: build run up down migrate clean

build:
	cd services/ingestion && go build -o ../../bin/ingestion-service ./cmd/main.go

run: build
	CONFIG_PATH=./config.yaml ./bin/ingestion-service scrape --url $(URL)

up:
	docker compose up -d

down:
	docker compose down

db-logs:
	docker compose logs -f db

clean:
	rm -rf bin/
