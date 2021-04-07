ldflags = -X 'main.githash=`git rev-parse --short HEAD`' \
          -X 'main.builddate=`date`'

# all builds a binary with the current commit hash
all:
	go install -ldflags "$(ldflags)" ./cmd/...

# static is like all, but for static binaries
static:
	go install -ldflags "$(ldflags) -s -w -extldflags='-static'" -tags='timetzdata' ./cmd/...

# dev builds a binary with dev constants
dev:
	go install -ldflags "$(ldflags)" -tags='dev' ./cmd/...

test:
	go test -short ./...

test-long:
	go test -v -race ./...

bench:
	go test -v -run=XXX -bench=. ./...

lint:
	@golint ./...
	@golangci-lint run \
		--enable-all \
		--disable=lll \
		--disable=gocyclo \
		--disable=prealloc \
		--disable=interfacer \
		--disable=unparam \
		--disable=gocritic \
		--disable=dupl \
		--disable=errcheck \
		--disable=gochecknoglobals \
		--disable=funlen \
		--skip-dirs=internal \
		./...

.PHONY: all static dev test test-long bench lint
