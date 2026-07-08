# CodeSteward Makefile
#
# Reproducible builds: version metadata is injected into
# internal/version via -ldflags. All builds use -trimpath so absolute
# paths never leak into binaries.

PKG        := github.com/codesteward-ai/codesteward
VERSION_PKG := $(PKG)/internal/version
BIN        := bin/codesteward
DIST       := dist

# Version metadata with deterministic fallbacks.
VERSION ?= $(shell git describe --tags --dirty 2>/dev/null || echo 0.1.0-dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(if $(SOURCE_DATE_EPOCH),$(shell date -u -d @$(SOURCE_DATE_EPOCH) +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -r $(SOURCE_DATE_EPOCH) +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown),unknown)

LDFLAGS := -s -w \
	-X $(VERSION_PKG).Version=$(VERSION) \
	-X $(VERSION_PKG).Commit=$(COMMIT) \
	-X $(VERSION_PKG).Date=$(DATE)

GO      ?= go
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: all build test vet fmt fmt-check build-all clean install

all: build

build:
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/codesteward

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

fmt-check:
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then \
		echo "gofmt needs to be run on:"; \
		echo "$$out"; \
		exit 1; \
	fi

build-all: clean-dist
	@mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		out="$(DIST)/codesteward_$${os}_$${arch}$${ext}"; \
		echo "building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o "$$out" ./cmd/codesteward || exit 1; \
	done
	@cd $(DIST) && \
		if command -v sha256sum >/dev/null 2>&1; then \
			sha256sum codesteward_* > SHA256SUMS; \
		else \
			shasum -a 256 codesteward_* > SHA256SUMS; \
		fi
	@echo "wrote $(DIST)/SHA256SUMS"

install:
	$(GO) install -trimpath -ldflags '$(LDFLAGS)' ./cmd/codesteward

.PHONY: clean-dist
clean-dist:
	rm -rf $(DIST)

clean:
	rm -rf $(BIN) bin $(DIST)
