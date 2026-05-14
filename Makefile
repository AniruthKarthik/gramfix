# GramFix Makefile
# Builds all gramfix binaries, installs services, and runs benchmarks.

BINARY_NAME   := gramfix
HOTKEY_NAME   := gramfix-hotkey
CHECK_NAME    := gramfix-check
BUILD_DIR     := build
INSTALL_DIR   ?= $(HOME)/.local/bin

GO            := go
GOFLAGS       := -trimpath
LDFLAGS       := -s -w

.PHONY: all build clean install uninstall test vet fmt lt-server bench

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
	@mkdir -p $(HOME)/.config/gramfix
	@if [ ! -f $(HOME)/.config/gramfix/.env ]; then \
		echo "--- Groq API Setup ---"; \
		read -p "Enter your GROQ_API_KEY (leave empty to skip): " key; \
		if [ ! -z "$$key" ]; then \
			echo "GROQ_API_KEY=$$key" > $(HOME)/.config/gramfix/.env; \
			echo "Saved to $(HOME)/.config/gramfix/.env"; \
		fi; \
	else \
		echo "Config folder already exists: $(HOME)/.config/gramfix"; \
	fi
	@echo ""
	@echo "Log file: $(HOME)/.local/share/gramfix/gramfix.log"
	@echo "Log format: date, time, sentence, method, corrected version"
	@echo ""
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

## lt-server: install optional local LanguageTool HTTP server as a systemd user service
lt-server:
	@mkdir -p $(HOME)/.config/systemd/user
	cp scripts/gramfix-lt-server.service $(HOME)/.config/systemd/user/gramfix-lt-server.service
	systemctl --user daemon-reload
	systemctl --user enable --now gramfix-lt-server.service
	@echo "gramfix-lt-server service enabled (port 8081)"
	@echo "Set GRAMFIX_LT_SERVER_URL=http://localhost:8081 in configs/gramfix.conf to use it"

## lt-server-stop: disable the optional LanguageTool HTTP server
lt-server-stop:
	-systemctl --user disable --now gramfix-lt-server.service 2>/dev/null
	-rm -f $(HOME)/.config/systemd/user/gramfix-lt-server.service
	-systemctl --user daemon-reload 2>/dev/null
	@echo "gramfix-lt-server service removed"

## bench: run accuracy benchmark against testdata/corpus.txt
bench: build
	@bash scripts/bench.sh

## uninstall: remove binaries and systemd service
uninstall:
	-systemctl --user disable --now gramfix-hotkey.service 2>/dev/null
	-systemctl --user disable --now gramfix-lt-server.service 2>/dev/null
	-rm -f $(HOME)/.config/systemd/user/gramfix-hotkey.service
	-rm -f $(HOME)/.config/systemd/user/gramfix-lt-server.service
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
