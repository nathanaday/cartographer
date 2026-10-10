BIN := carto
VERSION ?= dev
LDFLAGS := -ldflags "-X github.com/nathanaday/cartographer/internal/cli.Version=$(VERSION)"

.PHONY: build install test vet example plugin e2e

build:
	go build $(LDFLAGS) -o build/$(BIN) ./cmd/$(BIN)

# install puts the binary where the plugin's launcher (bin/carto) finds it.
install:
	mkdir -p $(HOME)/.cartographer/bin
	go build $(LDFLAGS) -o $(HOME)/.cartographer/bin/$(BIN) ./cmd/$(BIN)

vet:
	go vet ./...

test: vet
	go test ./...

# example renders the test vault the way the design example is rendered.
example:
	go run ./cmd/$(BIN) render testdata/vault/MyShop --sim testdata/prototype/simulation.yaml --examples

# plugin builds the Obsidian plugin into obsidian/dist and runs its unit tests.
plugin:
	cd obsidian && npm install --no-audit --no-fund && npm test && npm run build

# e2e runs the Obsidian plugin in a separate Obsidian with its own profile and vault.
e2e:
	cd obsidian && npm run test:obsidian
