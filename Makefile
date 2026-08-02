VERSION ?= dev
GO ?= go
NPM ?= npm
INSTALLER_VERSION = $(if $(filter dev,$(VERSION)),latest,$(VERSION))

.PHONY: all web test build release installer clean
all: test build

web:
	cd web && $(NPM) ci && $(NPM) run build

test:
	$(GO) test ./...
	cd web && $(NPM) run lint

build: web
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w -X main.buildVersion=$(VERSION)" -o dist/anpanel ./cmd/anpanel

release: web
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w -X main.buildVersion=$(VERSION)" -o dist/anpanel-linux-amd64 ./cmd/anpanel
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags "-s -w -X main.buildVersion=$(VERSION)" -o dist/anpanel-linux-arm64 ./cmd/anpanel
	cd dist && sha256sum anpanel-linux-amd64 > anpanel-linux-amd64.sha256
	cd dist && sha256sum anpanel-linux-arm64 > anpanel-linux-arm64.sha256
	$(MAKE) installer

installer:
	ANPANEL_INSTALLER_VERSION="$(INSTALLER_VERSION)" bash scripts/build-installer.sh dist/install.sh

clean:
	rm -rf dist web/node_modules .gocache .gomodcache
