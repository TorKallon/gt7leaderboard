COMPOSE := docker compose -f docker-compose.dev.yml

.PHONY: up down logs status rebuild test clean

## Docker ----------------------------------------------------------------

up: ## Start everything (foreground, with --build)
	$(COMPOSE) up --build

down: ## Stop and remove containers
	$(COMPOSE) down

logs: ## Tail logs (-f)
	$(COMPOSE) logs -f

status: ## Show container status
	$(COMPOSE) ps

rebuild: ## Stop, remove, build, and start all containers (background)
	$(COMPOSE) down
	$(COMPOSE) up --build -d

## Go / Collector --------------------------------------------------------

APP_BUNDLE := local-service/GT7Collector.app
APP_BIN    := $(APP_BUNDLE)/Contents/MacOS/collector

build: ## Build collector into .app bundle and codesign
	cd local-service && go build -o GT7Collector.app/Contents/MacOS/collector ./cmd/collector
	codesign -s - -f --entitlements local-service/entitlements.plist $(APP_BUNDLE)

collector: build ## Build and run the collector
	$(APP_BIN) --config local-service/config.dev.yaml

test: ## Run Go tests
	cd local-service && go test ./...

## Cleanup ---------------------------------------------------------------

clean: ## Remove old binary artifacts
	rm -f local-service/collector
	rm -f local-service/update-data
	rm -rf local-service/bin
	rm -f $(APP_BIN)
