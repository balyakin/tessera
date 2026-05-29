BIN ?= tessera

.PHONY: build test lint cover cross release docker bench

build:
	go build -o bin/$(BIN) ./cmd/tessera

test:
	go test ./...

lint:
	go vet ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

cross:
	GOOS=linux GOARCH=amd64 go build -o dist/tessera-linux-amd64 ./cmd/tessera
	GOOS=linux GOARCH=arm64 go build -o dist/tessera-linux-arm64 ./cmd/tessera
	GOOS=darwin GOARCH=amd64 go build -o dist/tessera-darwin-amd64 ./cmd/tessera
	GOOS=darwin GOARCH=arm64 go build -o dist/tessera-darwin-arm64 ./cmd/tessera
	GOOS=windows GOARCH=amd64 go build -o dist/tessera-windows-amd64.exe ./cmd/tessera

release: test cross
	cd dist && sha256sum * > checksums.txt

docker:
	docker build -t ghcr.io/balyakin/tessera:latest .

bench:
	go test -bench=. ./...
