VERSION ?= $(shell git describe --abbrev=4 --dirty --always --tags)
GO ?= go
LDFLAGS ?= -s -w -X main.BuildVersion=$(VERSION)

all: yamf

yamf: dep $(wildcard app.go executor/*.go scheduler/*.go internal/*/*.go)
	$(GO) build --ldflags "$(LDFLAGS)" -o yamf app.go

dep:
	@which dep 2>/dev/null || $(GO) get github.com/golang/dep/cmd/dep
	dep ensure

clean:
	rm -f ./yamf
