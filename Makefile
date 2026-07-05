BINARY  := regionlock
PKG     := ./cmd/regionlock
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

.PHONY: build test vet tidy lint-chart demo evidence clean

build: ## build the CLI
	go build $(LDFLAGS) -o $(BINARY) $(PKG)

test: ## run the test suite
	go test ./... -race

vet:
	go vet ./...

tidy:
	go mod tidy

lint-chart: ## requires helm
	helm lint chart/regionlock
	helm template regionlock chart/regionlock >/dev/null && echo "chart renders OK"

demo: ## full kind + kyverno + enforce demo
	./demo/run.sh

evidence: build ## generate a sample evidence report from the violating fixtures
	./$(BINARY) report --manifests testdata/violating --format console,html,md,json --out docs/sample

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf evidence
