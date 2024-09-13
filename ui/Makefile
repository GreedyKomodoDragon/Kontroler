# Project-specific variables
VERSION ?= 0.0.1
IMAGE_TAG_BASE ?= greedykomodo/kontroler-ui
IMG ?= $(IMAGE_TAG_BASE):$(VERSION)

# Container tool, change if using podman or other tools
CONTAINER_TOOL ?= docker

# Directory to store the built SolidJS app
DIST_DIR = dist

# Setting SHELL to bash for better error handling
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build docker-build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: install
install: ## Install the Node.js dependencies
	npm install

.PHONY: build
build: install ## Build the SolidJS project
	npm run build

##@ Docker

.PHONY: docker-build
docker-build: ## Build the Docker image for the SolidJS project
	$(CONTAINER_TOOL) build -t $(IMG) .

.PHONY: docker-push
docker-push: ## Push the Docker image to the remote repository
	$(CONTAINER_TOOL) push $(IMG)

.PHONY: clean
clean: ## Clean the project by removing the build directory
	rm -rf $(DIST_DIR)

