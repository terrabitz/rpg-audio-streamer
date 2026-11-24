EMBEDME := npx --yes embedme@1.22.1
EXAMPLE_DOCS := README.md INSTALLATION.md
IMAGE ?= rpg-audio-streamer:local
DOCKER_BUILD_FLAGS ?= --load --tag $(IMAGE)

.PHONY: dev devcontainer-build docker-build test-dev ui go test test-go test-go-discord test-ui test-frontend test-e2e test-e2e-discord test-examples verify-examples lint-ui build-ui record-e2e test-readme-demo gen gen-sqlc gen-api-client gen-docs gen-examples

ui:
	cd ui && npm install && npm run dev

go:
	air

dev:
	node scripts/dev.mjs

devcontainer-build:
	devcontainer build --workspace-folder .

docker-build:
	docker buildx build --file docker/Dockerfile $(DOCKER_BUILD_FLAGS) .

test-dev:
	node --test scripts/dev.test.mjs scripts/discord-env.test.mjs

test: test-go test-ui

test-go:
	go test -count=1 -race -coverpkg=./internal/... -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | awk '/^total:/ { print; found=1; if ($$3+0 < 80) { print "Internal Go coverage must be at least 80%"; exit 1 } } END { if (!found) exit 1 }'

test-go-discord:
	DISCORD_LIVE_INTEGRATION=1 go test -count=1 -race ./internal/discord -run '^TestLiveVoiceConnection$$'

test-ui:
	node --test ui/e2e/discord-config.test.ts ui/e2e/discord-listener.test.ts ui/e2e/youtube-config.test.ts
	cd ui && npm run test:coverage

test-frontend: test-ui test-dev

test-e2e:
	cd ui && npm run test:e2e -- --project=chromium --project=chromium-timing --project=chromium-youtube

test-e2e-discord:
	cd ui && npm run test:e2e -- --project=chromium-discord

test-examples:
	mkdir -p tmp
	TMPDIR="$(CURDIR)/tmp" node --test examples/examples.test.mjs

verify-examples:
	$(EMBEDME) --verify $(EXAMPLE_DOCS)

lint-ui:
	npm --prefix ui run lint:check

build-ui:
	npm --prefix ui run build

record-e2e:
	cd ui && npm run test:e2e:record -- $(if $(GREP),--grep "$(GREP)")

README_DEMO_CHECKPOINTS = media/readme-demo/gm.png media/readme-demo/player.png
README_DEMO_STAMP = tmp/readme-demo.stamp

test-readme-demo:
	mkdir -p tmp
	touch $(README_DEMO_STAMP)
	cd ui && E2E_README_DEMO=1 npm run test:e2e:record -- readme-demo.spec.ts --project=chromium --update-snapshots=changed
	if [ -n "$$(find $(README_DEMO_CHECKPOINTS) -newer $(README_DEMO_STAMP))" ]; then cp ui/recordings/gm-switches-music-while-a-player-listens.gif media/audio-playing.gif; fi

gen: gen-sqlc gen-api-client gen-docs gen-examples

gen-sqlc:
	sqlc generate

gen-api-client:
	cd ui && npm run generate-api-client

gen-docs:
	cd ui && npm run generate-docs

gen-examples:
	$(EMBEDME) $(EXAMPLE_DOCS)
