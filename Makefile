.PHONY: default
default: build
module=pubcrawl
BINDIR ?= /usr/local/bin
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
GO_VERSION = $(shell curl -s https://go.dev/dl/?mode=json | jq -r '.[0].version' | sed 's/go//')
ALPINE_VERSION=3.24

.PHONY: clean
clean:
	@if [ -e "./$(module)" ]; then rm "./$(module)"; fi
	@if [ -e vendor ]; then rm -rf vendor; fi
	@if [ -e cover.out ]; then rm cover.out; fi

.PHONY: update-packages
update-packages:
	go mod edit -go=${GO_VERSION}
	go get -u ./...
	go mod tidy

.PHONY: fix
fix:
	go fix ./...

.PHONY: lint
lint:
	golangci-lint run 

.PHONY: lint-strict
lint-strict:
	gocritic check -enableAll ./...
	
.PHONY: run
run:
	go run .
	
.PHONY: test
test:
	go test ./...

.PHONY: test-integration
test-integration:
	go test -tags=integration -count=1 ./...

.PHONY: test-clean
test-clean:
	go clean -testcache
	go test ./...

.PHONY: test-cover
test-cover:
	go test ./... -coverprofile cover.out
	go tool cover -html=cover.out

.PHONY: build
build:
	go build -buildvcs=false -o ${module}  -ldflags="-X main.Version=${GIT_COMMIT}"

.PHONY: race
race:
	go build -buildvcs=false -race -o ${module}

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: install
install: build test lint
	sudo install -d "$(DESTDIR)$(BINDIR)"
	sudo install -m 0755 "$(module)" "$(DESTDIR)$(BINDIR)/$(module)"

.PHONY: test-fuzz
test-fuzz:
	go test ./internal/pubcrawl -fuzz='^FuzzValidate$$' -fuzztime=60s

.PHONY: pre-image post-image docker image
pre-image:
	go mod vendor

post-image:
	rm -rf vendor

docker:
	docker build -t $(module) --build-arg revision=$(GIT_COMMIT) .

image: tidy
	docker build -t $(module):$(GIT_COMMIT) --build-arg alpine_version=$(ALPINE_VERSION) --build-arg revision=$(GIT_COMMIT) .
