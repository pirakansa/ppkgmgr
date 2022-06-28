# Makefile

PROJECT_NAME= ppkgmgr
BINDIR      = ./bin
LINUX_AMD64 = linux-amd64
LINUX_ARM   = linux-arm
LINUX_ARM64 = linux-arm64
WIN_AMD64   = win-amd64
SRCDIR      = ./cmd/$(PROJECT_NAME)
VERSION     = 0.1.0
GO_LDFLAGS  = -ldflags="-s -w -X main.Version=$(VERSION)"


all: build

.PHONY: build
build:
	@go build -o $(BINDIR)/host/ $(SRCDIR)

release: $(LINUX_AMD64) $(LINUX_ARM) $(LINUX_ARM64)

$(LINUX_AMD64):
	@GOOS=linux   GOARCH=amd64 go build $(GO_LDFLAGS) -o $(BINDIR)/$(LINUX_AMD64)/   $(SRCDIR)
	@tar --gunzip --create --directory=$(BINDIR)/$(LINUX_AMD64)/ --file=./$(PROJECT_NAME)_$(LINUX_AMD64).tar.gz .
$(LINUX_ARM):
	@GOOS=linux   GOARCH=arm   go build $(GO_LDFLAGS) -o $(BINDIR)/$(LINUX_ARM)/     $(SRCDIR)
	@tar --gunzip --create --directory=$(BINDIR)/$(LINUX_ARM)/   --file=./$(PROJECT_NAME)_$(LINUX_ARM).tar.gz .
$(LINUX_ARM64):
	@GOOS=linux   GOARCH=arm64 go build $(GO_LDFLAGS) -o $(BINDIR)/$(LINUX_ARM64)/   $(SRCDIR)
	@tar --gunzip --create --directory=$(BINDIR)/$(LINUX_ARM64)/ --file=./$(PROJECT_NAME)_$(LINUX_ARM64).tar.gz .
$(WIN_AMD64):
	@GOOS=windows GOARCH=amd64 go build $(GO_LDFLAGS) -o $(BINDIR)/$(WIN_AMD64)/     $(SRCDIR)
	@tar --gunzip --create --directory=$(BINDIR)/$(WIN_AMD64)/   --file=./$(PROJECT_NAME)_$(WIN_AMD64).tar.gz .

debug: 
	@go run -tags=debug $(SRCDIR)

.PHONY: test
test: 
	@go test -v ./...

clean:
	@rm -fr $(BINDIR)
	@rm -f  ./$(PROJECT_NAME)_*.tar.gz
	@go clean

