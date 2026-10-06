GO ?= go

.PHONY: build install fmt-check check test snapshot
build:
	mkdir -p work/bin
	CGO_ENABLED=0 $(GO) build -mod=readonly -trimpath -o work/bin/note ./cmd/note
install:
	$(GO) install -mod=readonly ./cmd/note
fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || { gofmt -l cmd internal; exit 1; }
test:
	$(GO) test -mod=readonly ./...
check: fmt-check
	$(GO) mod verify
	$(GO) vet -mod=readonly ./...
	$(GO) test -mod=readonly ./...
	CGO_ENABLED=1 $(GO) test -mod=readonly -race ./...
snapshot:
	$(GO) run -mod=readonly ./cmd/release --version snapshot
