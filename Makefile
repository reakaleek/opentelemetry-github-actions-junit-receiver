SHELL := bash
MAKEFLAGS += --no-print-directory
WEBHOOK_SECRET?=
GITHUB_TOKEN?=

## MAKE GOALS
.PHONY: build
build: ## Build the binary
	@ocb --config builder-config.yml

.PHONY: run
run: ## Run the binary
	@WEBHOOK_SECRET=$(WEBHOOK_SECRET) \
	GITHUB_TOKEN=$(GITHUB_TOKEN) \
	./bin/otelcol-custom --config config.yml
