SHELL := bash
MAKEFLAGS += --no-print-directory
WEBHOOK_SECRET ?= secret
GITHUB_TOKEN ?= $(shell gh auth token)

## MAKE GOALS
.PHONY: build
build: ## Build the binary
	@ocb --config builder-config.yml

.PHONY: run
run: ## Run the binary
	@WEBHOOK_SECRET=$(WEBHOOK_SECRET) \
	GITHUB_TOKEN=$(GITHUB_TOKEN) \
	./bin/otelcol-custom --config config.yml

.PHONY: ngrok
ngrok: ## Run ngrok
	ngrok http http://localhost:33333
