.PHONY: build serve translate up down db-logs clean

build:
	cd services/ingestion && go build -o ../../bin/ingestion-service ./cmd/main.go

serve: build
	CONFIG_PATH=./config.yaml ./bin/ingestion-service serve

translate: build
	CONFIG_PATH=./config.yaml ./bin/ingestion-service translate

up:
	docker compose up -d

down:
	docker compose down

db-logs:
	docker compose logs -f db

clean:
	rm -rf bin/
