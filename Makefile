.PHONY: test build demo demo-overview demo-themes demo-resources demo-batch-sync demo-inspect

test:
	go vet ./...
	go test ./...

build:
	go build -o dist/argoxd ./cmd/argoxd

# Re-record demo GIFs from the built-in sample data (needs vhs).
demo: demo-overview demo-themes demo-resources demo-batch-sync demo-inspect

demo-overview: build
	PATH="$(CURDIR)/dist:$$PATH" vhs demo/demo.tape

demo-themes: build
	PATH="$(CURDIR)/dist:$$PATH" vhs demo/themes.tape

demo-resources: build
	PATH="$(CURDIR)/dist:$$PATH" vhs demo/resources.tape

demo-batch-sync: build
	PATH="$(CURDIR)/dist:$$PATH" vhs demo/batch-sync.tape

demo-inspect: build
	PATH="$(CURDIR)/dist:$$PATH" vhs demo/inspect.tape
