BINARY  := regionlock
PKG     := ./cmd/regionlock
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

.PHONY: build test vet lint tidy lint-chart gator-test demo evidence gen-policies snapshot docs docs-serve clean

build: ## build the CLI
	go build $(LDFLAGS) -o $(BINARY) $(PKG)

test: ## run the test suite
	go test ./... -race

vet:
	go vet ./...

lint: ## static analysis (requires golangci-lint)
	golangci-lint run

tidy:
	go mod tidy

lint-chart: ## requires helm
	helm lint chart/regionlock
	@for e in kyverno gatekeeper both; do \
		helm template regionlock chart/regionlock --set engine=$$e >/dev/null && echo "engine=$$e renders OK"; \
	done

gator-test: ## test the Gatekeeper Rego (requires helm + gator)
	helm template rl chart/regionlock --set engine=gatekeeper > /tmp/gk.yaml
	gator test -f /tmp/gk.yaml -f chart/regionlock/gatekeeper-tests/resources.yaml

demo: ## full kind + kyverno + enforce demo
	./demo/run.sh

evidence: build ## regenerate the sample evidence report from the violating fixtures
	./$(BINARY) report --manifests testdata/violating --format console,html,md,json,pdf,sarif,prometheus,oscal --out docs/sample

gen-policies: ## regenerate the embedded `regionlock policy` templates from the chart (requires helm)
	helm template regionlock chart/regionlock --set engine=kyverno --set-json 'euRegions=["__RL_REGIONS__"]' \
	  | sed -e 's/                  - __RL_REGIONS__/«regions 18»/' \
	        -e 's#regionlock.io/ruleset: eu-data-residency-v1#regionlock.io/ruleset: «.RulesetID»#' \
	        -e 's#app.kubernetes.io/managed-by: Helm#app.kubernetes.io/managed-by: regionlock#' \
	  > internal/policygen/kyverno.yaml.tmpl
	helm template regionlock chart/regionlock --set engine=gatekeeper --set-json 'euRegions=["__RL_REGIONS__"]' \
	  | sed -e 's/      - __RL_REGIONS__/«regions 6»/' \
	        -e 's#regionlock.io/ruleset: eu-data-residency-v1#regionlock.io/ruleset: «.RulesetID»#' \
	        -e 's#app.kubernetes.io/managed-by: Helm#app.kubernetes.io/managed-by: regionlock#' \
	  > internal/policygen/gatekeeper.yaml.tmpl

snapshot: ## build a local release snapshot (requires goreleaser)
	goreleaser release --snapshot --clean

docs: ## build the documentation site (requires mkdocs-material)
	mkdocs build --strict

docs-serve: ## live-preview the docs site at http://127.0.0.1:8000
	mkdocs serve

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf evidence dist site
