.DEFAULT_GOAL := help
M = $(shell printf "\033[34;1m>>\033[0m")
rwildcard = $(foreach d,$(wildcard $(1:=/*)),$(call rwildcard,$d,$2) $(filter $(subst *,%,$2),$d))

SHELL            := /bin/sh
TOOLS            ?= $(PWD)/.tools
PATH             := $(TOOLS):$(PATH)
GO               = go
VERSION          := $(shell git describe --tags)
DATE             := $(shell date +%FT%T%z)
VCS_REVISION     := $(shell git rev-parse HEAD)
VCS_BRANCH       := $(shell git rev-parse --abbrev-ref HEAD)
LDFLAGS          = -ldflags "-X 'main.buildRelease=$(VERSION)' \
                             -X 'main.vscRevision=$(VCS_REVISION)' \
                             -X 'main.vcsBranch=$(VCS_BRANCH)' \
                             -X 'main.buildDate=$(DATE)'"

TARGET_DIR       ?= $(PWD)/.build
RELEASE_DIR       ?= $(PWD)/.release
DEMO_DIR	 ?= $(PWD)/demo

TARGET_NAME = $(firstword $(MAKECMDGOALS))
RUN_ARGS = $(filter-out $@, $(MAKEOVERRIDES) $(MAKECMDGOALS))

.PHONY: version-info
version-info:
	@echo VERSION: $(VERSION), Revision: $(VCS_REVISION), Branch: $(VCS_BRANCH), BuildDate: $(DATE)

.PHONY: install-tools
install-tools: $(TOOLS) ## Install tools needed for development
	$(info $(M) Install tools needed for development...)
	@GOBIN=$(TOOLS) $(GO) install \
		github.com/golangci/golangci-lint/cmd/golangci-lint \
		golang.org/x/tools/cmd/goimports

.PHONY: fmt
fmt: ## Format code
	$(info $(M) running gofmt...)
	$(eval $@_module := $(shell $(GO) list -m))
	@ret=0 && for d in $$($(GO) list -f '{{.Dir}}' ./... | grep -v '/vendor\|/mock\|/proto'); do \
		$(GO) run golang.org/x/tools/cmd/goimports -local '$($@_module)' -w -format-only $$d/*.go || ret=$$? ; \
		done ; exit $$ret

.PHONY: build
build: ## Build app
	$(info $(M) Build go code...)
	@GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(GCFLAGS) $(LDFLAGS) -o $(TARGET_DIR)/andy *.go

.PHONY: build-release
BINARY_NAME := andy_$(GOOS)_$(GOARCH)
ARCHIVE_NAME := $(BINARY_NAME).tar.gz
ifeq ($(GOOS),windows)
	ARCHIVE_NAME := $(BINARY_NAME).zip
	BINARY_NAME := $(BINARY_NAME).exe
endif
build-release: ## Build binary for release
	$(info $(M) Build binary for release $(BINARY_NAME)...)
	@mkdir -p $(RELEASE_DIR)
	GOARCH=$(GOARCH) $(GO) build $(GCFLAGS) $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME) *.go
	echo - Create packed $(ARCHIVE_NAME)
	@if [ "$(GOOS)" = "windows" ]; then \
		zip -j $(RELEASE_DIR)/$(ARCHIVE_NAME) $(RELEASE_DIR)/$(BINARY_NAME); \
	else \
		tar -czf $(RELEASE_DIR)/$(ARCHIVE_NAME) -C $(RELEASE_DIR) $(BINARY_NAME); \
	fi

.PHONY: build-release-pack
build-release-pack:
	GOOS=linux GOARCH=amd64 make build-release
	GOOS=darwin GOARCH=amd64 make build-release
	GOOS=windows GOARCH=amd64 make build-release
	GOOS=windows GOARCH=386 make build-release


.PHONY: test
test: ## Run all tests (pass package as argument if you want test specific one)
	$(eval @_scope := $(or $(addprefix './',$(filter-out $@,$(MAKECMDGOALS))), './...'))
	$(info $(M) running tests for $(@_scope))
	@$(GO) test '$(@_scope)' -v -tags mock,integration -race -cover

.PHONY: test-no-cache
test-no-cache: ## Run all tests withput cache (pass package as argument if you want test specific one)
	$(info $(M) Test go code without cache...)
	@$(GO) test '$(@_scope)' -v -tags mock,integration -race -cover -count=1

.PHONY: lint
lint: install-tools ## Run code linters
	$(info $(M) Lint go files...)
	@$(TOOLS)/golangci-lint run ${args}

.PHONY: lint-fix
lint-fix: install-tools ## Do  fixes for linter wanings
	$(info $(M) fixing linter issues...)
	@$(TOOLS)/golangci-lint run --fix --verbose --timeout 2m0s ./... 2>&1 | \
		awk 'BEGIN{FS="="} /Fix/ { print $$3}' | \
		awk 'BEGIN{FS=","} {print " * ", $$1, $$2, $$8, $$9, $$10, $$11}' | \
		sed 's/\\"/"/g' | sed -e 's/&result.Issue{//g' | sed 's/token.Position//'

.PHONY: prepare-demo
prepare-demo: $(DEMO_DIR) build ## Preapres demo
	$(info $(M) Preapre demo...)
	@cp $(TARGET_DIR)/andy $(DEMO_DIR)/andy
	@echo "Rady for demo. Go to demo dir and run examples"

.PHONY: $(TOOLS)
$(TOOLS):
	@mkdir -p $(TOOLS)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

%:
	@:
