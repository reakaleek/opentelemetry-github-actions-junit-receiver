SHELL := bash
MAKEFLAGS += --no-print-directory
WEBHOOK_SECRET ?= secret
GITHUB_TOKEN ?= $(shell gh auth token)

#######################
## Tools
#######################
export PATH := $(CURDIR)/bin:$(PATH)
ARCH = $(shell uname -m)
OS = $(shell uname)
ifeq ($(ARCH),x86_64)
	OCB_ARCH ?= amd64
else
	OCB_ARCH ?= arm64
endif
ifeq ($(OS),Darwin)
	OCB_BINARY ?= darwin_$(OCB_ARCH)
else
	OCB_BINARY ?= yq_linux_$(OCB_ARCH)
endif
OCB ?= ocb

## @help:install-ngrok:Install ngrok.
.PHONY: install-ngrok
install-ngrok:
ifeq ($(OS),Darwin)
	brew install ngrok/ngrok/ngrok
else
	$(error "Please install ngrok manually")
endif

## @help:install-ocb:Install ocb.
.PHONY: install-ocb
install-ocb:
	curl --proto '=https' --tlsv1.2 -fL -o $(CURDIR)/bin/$(OCB) \
	https://github.com/open-telemetry/opentelemetry-collector/releases/download/cmd%2Fbuilder%2Fv0.102.1/ocb_0.102.1_$(OCB_BINARY)
	chmod +x $(CURDIR)/bin/$(OCB)

## MAKE GOALS
.PHONY: build
build: ## Build the binary
	@$(OCB) --config builder-config.yml

.PHONY: run
run: ## Run the binary
	@WEBHOOK_SECRET=$(WEBHOOK_SECRET) \
	GITHUB_TOKEN=$(GITHUB_TOKEN) \
	./bin/otelcol-custom --config config.yml

.PHONY: ngrok
ngrok: ## Run ngrok
	ngrok http http://localhost:33333
