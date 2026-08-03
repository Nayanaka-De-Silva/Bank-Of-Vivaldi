# Tailwind is a committed, generated artifact (see web/tailwind/input.css). The
# standalone tailwindcss binary keeps the toolchain Node-free, matching the rest
# of this Go-stdlib-heavy repo.
TAILWIND_VERSION ?= v4.1.11
TAILWIND_OS      ?= linux
TAILWIND_ARCH    ?= x64
TAILWIND_BIN     := bin/tailwindcss
TAILWIND_URL     := https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-$(TAILWIND_OS)-$(TAILWIND_ARCH)

.PHONY: css css-watch test build

$(TAILWIND_BIN):
	mkdir -p bin
	curl -sSfL -o $(TAILWIND_BIN) $(TAILWIND_URL)
	chmod +x $(TAILWIND_BIN)

css: $(TAILWIND_BIN) ## Regenerate web/static/app.css from web/tailwind/input.css. Commit the result.
	$(TAILWIND_BIN) -i web/tailwind/input.css -o web/static/app.css

css-watch: $(TAILWIND_BIN) ## Rebuild web/static/app.css on every template/input.css change.
	$(TAILWIND_BIN) -i web/tailwind/input.css -o web/static/app.css --watch

test: ## Run the Go test suite inside the dev container.
	docker compose run --rm app go test ./...

build: ## Compile the server binary (embeds web/static/app.css at build time).
	go build ./cmd/...
