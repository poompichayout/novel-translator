.PHONY: build run up down migrate clean

build:
	cd services/ingestion && go build -o ../../bin/ingestion-service ./cmd/main.go
	python3 -m venv .venv
	.venv/bin/pip install -r scripts/scrapegraph/requirements.txt
	.venv/bin/python -m playwright install chromium

run: build
	CONFIG_PATH=./config.yaml ./bin/ingestion-service scrape --url $(URL) --source-lang $(if $(SL),$(SL),en) --target-lang $(if $(TL),$(TL),th)

up:
	docker compose up -d

down:
	docker compose down

db-logs:
	docker compose logs -f db

clean:
	rm -rf bin/
