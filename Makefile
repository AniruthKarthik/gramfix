# GramFix Makefile
# Builds both the gramfix pipeline binary and the gramfix-hotkey daemon.

BINARY_NAME   := gramfix
HOTKEY_NAME   := gramfix-hotkey
CHECK_NAME    := gramfix-check
BUILD_DIR     := build
INSTALL_DIR   ?= $(HOME)/.local/bin

GO            := go
GOFLAGS       := -trimpath
LDFLAGS       := -s -w

.PHONY: all build clean install uninstall test vet fmt

all: build

## build: compile all binaries into build/
build:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/gramfix
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(HOTKEY_NAME) ./cmd/gramfix-hotkey
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(CHECK_NAME) ./cmd/gramfix-check
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME), $(BUILD_DIR)/$(HOTKEY_NAME), $(BUILD_DIR)/$(CHECK_NAME)"

## install: install binaries to INSTALL_DIR (default: ~/.local/bin)
install: build
	@mkdir -p $(INSTALL_DIR)
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME)  $(INSTALL_DIR)/$(BINARY_NAME)
	install -m 755 $(BUILD_DIR)/$(HOTKEY_NAME)  $(INSTALL_DIR)/$(HOTKEY_NAME)
	install -m 755 $(BUILD_DIR)/$(CHECK_NAME)   $(INSTALL_DIR)/$(CHECK_NAME)
	@echo "Installed to $(INSTALL_DIR)"
	@echo "Run 'make systemd' to install the systemd user service"

## systemd: install and enable the systemd user service for the hotkey daemon
systemd: install
	@mkdir -p $(HOME)/.config/systemd/user
	@sed \
		-e 's|__HOTKEY_BIN__|$(INSTALL_DIR)/$(HOTKEY_NAME)|g' \
		-e 's|__GRAMFIX_BIN__|$(INSTALL_DIR)/$(BINARY_NAME)|g' \
		scripts/gramfix-hotkey.service.template \
		> $(HOME)/.config/systemd/user/gramfix-hotkey.service
	systemctl --user daemon-reload
	systemctl --user enable --now gramfix-hotkey.service
	@echo "gramfix-hotkey service enabled and started"

## uninstall: remove binaries and systemd service
uninstall:
	-systemctl --user disable --now gramfix-hotkey.service 2>/dev/null
	-rm -f $(HOME)/.config/systemd/user/gramfix-hotkey.service
	-systemctl --user daemon-reload 2>/dev/null
	rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	rm -f $(INSTALL_DIR)/$(HOTKEY_NAME)
	@echo "Uninstalled"

## test: run unit tests
test:
	$(GO) test ./... -v -count=1 -timeout 120s

## vet: run go vet
vet:
	$(GO) vet ./...

## fmt: run gofmt
fmt:
	gofmt -w .

## clean: remove build artifacts
clean:
	rm -rf $(BUILD_DIR)

## help: show this help
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
