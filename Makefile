APP := footcd
DIST_DIR := build
GO ?= go
GOCACHE ?= /tmp/go-build
VERSION := 1.0.0
LDFLAGS := -s -w -X main.version=$(VERSION)

TARGETS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

.PHONY: build
build:
	GOCACHE=$(GOCACHE) $(GO) build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(APP) .

.PHONY: cross
cross: $(TARGETS:%=$(DIST_DIR)/%)

$(DIST_DIR)/%:
	@mkdir -p $(DIST_DIR)
	@os='$(word 1,$(subst /, ,$*))'; \
	arch='$(word 2,$(subst /, ,$*))'; \
	ext=''; \
	if [ "$$os" = "windows" ]; then ext='.exe'; fi; \
	out='$(DIST_DIR)/$(APP)-'$$os'-'$$arch"$$ext"; \
	echo "building $$out"; \
	GOCACHE=$(GOCACHE) GOOS="$$os" GOARCH="$$arch" $(GO) build -ldflags "$(LDFLAGS)" -o "$$out" .

.PHONY: clean
clean:
	rm -f $(APP)
	rm -rf $(DIST_DIR)
