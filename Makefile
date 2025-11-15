# Makefile

PROJECT_NAME := ppkgmgr
BINDIR       := ./bin
HOSTDIR      := $(BINDIR)/host
LINUX_AMD64  := linux-amd64
LINUX_ARM    := linux-arm
LINUX_ARM64  := linux-arm64
WIN_AMD64    := win-amd64
CMD_DIR      := ./cmd/$(PROJECT_NAME)
VERSION      := 0.3.0
GO_LDFLAGS   := -ldflags="-s -w -X main.Version=$(VERSION)" -trimpath
GO_TAGS      := osusergo netgo
GO_TAG_FLAGS := -tags="$(GO_TAGS)"

all: build

.PHONY: build
build:
	@mkdir -p $(HOSTDIR)
	@go build $(GO_TAG_FLAGS) -o $(HOSTDIR)/ $(CMD_DIR)

.PHONY: run
run:
	@go run $(GO_TAG_FLAGS) $(CMD_DIR)

.PHONY: release
release: $(LINUX_AMD64) $(LINUX_ARM) $(LINUX_ARM64)

$(LINUX_AMD64):
	@mkdir -p $(BINDIR)/$(LINUX_AMD64)
	@GOOS=linux   GOARCH=amd64 go build $(GO_TAG_FLAGS) $(GO_LDFLAGS) -o $(BINDIR)/$(LINUX_AMD64)/   $(CMD_DIR)
	@tar --gunzip --create --directory=$(BINDIR)/$(LINUX_AMD64)/ --file=./$(PROJECT_NAME)_$(LINUX_AMD64).tar.gz .
$(LINUX_ARM):
	@mkdir -p $(BINDIR)/$(LINUX_ARM)
	@GOOS=linux   GOARCH=arm   go build $(GO_TAG_FLAGS) $(GO_LDFLAGS) -o $(BINDIR)/$(LINUX_ARM)/     $(CMD_DIR)
	@tar --gunzip --create --directory=$(BINDIR)/$(LINUX_ARM)/   --file=./$(PROJECT_NAME)_$(LINUX_ARM).tar.gz .
$(LINUX_ARM64):
	@mkdir -p $(BINDIR)/$(LINUX_ARM64)
	@GOOS=linux   GOARCH=arm64 go build $(GO_TAG_FLAGS) $(GO_LDFLAGS) -o $(BINDIR)/$(LINUX_ARM64)/   $(CMD_DIR)
	@tar --gunzip --create --directory=$(BINDIR)/$(LINUX_ARM64)/ --file=./$(PROJECT_NAME)_$(LINUX_ARM64).tar.gz .
$(WIN_AMD64):
	@mkdir -p $(BINDIR)/$(WIN_AMD64)
	@GOOS=windows GOARCH=amd64 go build $(GO_TAG_FLAGS) $(GO_LDFLAGS) -o $(BINDIR)/$(WIN_AMD64)/     $(CMD_DIR)
	@tar --gunzip --create --directory=$(BINDIR)/$(WIN_AMD64)/   --file=./$(PROJECT_NAME)_$(WIN_AMD64).tar.gz .

.PHONY: debug
debug:
	@go run -tags="debug $(GO_TAGS)" $(CMD_DIR)

.PHONY: vet
vet:
	@go vet $(GO_TAG_FLAGS) ./...

.PHONY: staticcheck
staticcheck:
	@if ! command -v staticcheck >/dev/null 2>&1; then \
		go install honnef.co/go/tools/cmd/staticcheck@latest; \
	fi
	@staticcheck ./...

.PHONY: govulncheck
govulncheck:
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
	@govulncheck -tags "$(GO_TAGS)" ./...

.PHONY: lint
lint: vet staticcheck

.PHONY: test
test: lint
	@go test -race $(GO_TAG_FLAGS) -v ./...

.PHONY: clean
clean:
	@rm -fr $(BINDIR)
	@rm -f  ./$(PROJECT_NAME)_*.tar.gz
	@go clean
