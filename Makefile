.PHONY: web build install test

web:
	cd web && npm ci && npm run build && touch ../internal/webui/dist/.gitkeep

build: web
	go build -o bin/yaam ./cmd/yaam

install: web
	go install ./cmd/yaam
	"$$(go env GOPATH)/bin/yaam" install

test:
	go test ./...
