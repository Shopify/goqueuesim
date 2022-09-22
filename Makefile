GOCMD=go
GOLANGCI-LINT=golangci-lint
GORELEASER=goreleaser
GOPATH?=`echo $$GOPATH`
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test

DOCKER=docker
CODE=./cmd/
PACKAGES := goqueuesim
IMAGE_NAME?=goqueuesim

all: goqueuesim run

# System Dependencies
system-deps:
ifeq ($(shell $(GOCMD) version 2> /dev/null) , "")
	$(error "go is not installed")
endif
ifeq ($(shell $(GOLANGCI-LINT) version 2> /dev/null) , "")
	$(error "golangci-lint is not installed")
endif
ifeq ($(shell $(GORELEASER) --version dot 2> /dev/null) , "")
	$(error "goreleaser is not installed")
endif
	$(info "No missing dependencies")

ensure-deps:
	$(GOCMD) mod download
	$(GOCMD) mod verify

update-deps:
	$(GOCMD) get -u -t all
	$(GOCMD) mod tidy

goqueuesim: bin bin/goqueuesim

bin:
	mkdir -p bin

clean:
	$(GOCLEAN) ./cmd/...
	rm -rf bin

bin/goqueuesim:
	@printf "Compiling bin/goqueuesim\n"
	$(GOBUILD) -o $@ -v ./cmd/goqueuesim/...

run:
	./bin/goqueuesim

test:
	$(GOTEST) -v ./...

release:
	$(GORELEASER)

snapshot:
	$(GORELEASER) --snapshot

.PHONY: all buyersim goqueuesim bin/goqueuesim
