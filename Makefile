BINARY_NAME=7zkpxc
BUILD_DIR=bin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS = -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

PREFIX     ?= /usr/local
BINDIR     ?= $(PREFIX)/bin
ZSH_COMP   ?= $(PREFIX)/share/zsh/site-functions
BASH_COMP  ?= /etc/bash_completion.d
REAL_HOME  := $(shell eval echo ~$${SUDO_USER:-$$USER})
CONFIG_DIR ?= $(REAL_HOME)/.config/$(BINARY_NAME)

.PHONY: all build clean test run completion install uninstall purge clean-artifacts

all: build

build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/7zkpxc

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -rf completions
	@go clean

test:
	@go test ./...

run: build
	@./$(BUILD_DIR)/$(BINARY_NAME)

clean-artifacts:
	@rm -f secrets*.7z

completion: build
	@mkdir -p completions
	@./$(BUILD_DIR)/$(BINARY_NAME) completion zsh  > completions/_$(BINARY_NAME)
	@./$(BUILD_DIR)/$(BINARY_NAME) completion bash > completions/$(BINARY_NAME).bash
	@echo "Completions generated in completions/"

# ---------- System-wide install ----------
# Build BEFORE sudo so bin/ stays user-owned:
#   make build && sudo make install

install:
	@if [ ! -f $(BUILD_DIR)/$(BINARY_NAME) ]; then \
		echo ""; \
		echo "  Error: $(BUILD_DIR)/$(BINARY_NAME) not found."; \
		echo ""; \
		echo "  Build first (as your normal user), then install:"; \
		echo "    make build"; \
		echo "    sudo make install"; \
		echo ""; \
		exit 1; \
	fi
	@echo "Installing $(BINARY_NAME) to $(BINDIR)..."
	@install -Dm755 $(BUILD_DIR)/$(BINARY_NAME) $(BINDIR)/$(BINARY_NAME)

	@echo "Installing completions..."
	@mkdir -p $(ZSH_COMP)
	@$(BUILD_DIR)/$(BINARY_NAME) completion zsh > $(ZSH_COMP)/_$(BINARY_NAME)
	@echo "  -> zsh  $(ZSH_COMP)/_$(BINARY_NAME)"

	@if [ -d $(BASH_COMP) ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) completion bash > $(BASH_COMP)/$(BINARY_NAME); \
		echo "  -> bash $(BASH_COMP)/$(BINARY_NAME)"; \
	fi

	@echo ""
	@echo "Done! Restart your shell or run: hash -r"

# Remove binary + completions (keeps config)
uninstall:
	@echo "Removing $(BINARY_NAME)..."
	@rm -f $(BINDIR)/$(BINARY_NAME)
	@rm -f $(ZSH_COMP)/_$(BINARY_NAME)
	@rm -f $(BASH_COMP)/$(BINARY_NAME)
	@echo "Uninstalled (config left in $(CONFIG_DIR))."

# Remove everything: binary + completions + config
purge: uninstall
	@echo "Removing config directory $(CONFIG_DIR)..."
	@rm -rf $(CONFIG_DIR)
	@echo "Purged. No traces left."
