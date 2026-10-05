.PHONY: test build demo

test:
	go vet ./...
	go test ./...

build:
	go build -o dist/argoxd ./cmd/argoxd

# Re-record demo/demo.gif from the built-in sample data (needs vhs).
demo: build
	PATH="$(CURDIR)/dist:$$PATH" vhs demo/demo.tape
