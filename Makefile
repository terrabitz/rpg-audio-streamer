.PHONY: dev test-dev ui go test test-go test-ui test-discord test-e2e test-e2e-discord record-e2e gen gen-sqlc gen-api-client gen-docs

ui:
	cd ui && npm install && npm run dev

go:
	air

dev:
	node scripts/dev.mjs

test-dev:
	node --test scripts/dev.test.mjs scripts/discord-env.test.mjs

test: test-go test-ui test-discord

test-go:
	go test -count=1 -race -coverpkg=./internal/... -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | awk '/^total:/ { print; found=1; if ($$3+0 < 80) { print "Internal Go coverage must be at least 80%"; exit 1 } } END { if (!found) exit 1 }'

test-ui:
	cd ui && npm run test:coverage

test-e2e:
	cd ui && npm run test:e2e

test-discord:
	npm test --prefix discord
	node --test scripts/discord-env.test.mjs

test-e2e-discord:
	E2E_DISCORD=1 go test ./e2e/ -run TestDiscord -v -count=1 -timeout 5m
	cd ui && E2E_DISCORD=1 npm run test:e2e -- --grep 'Discord'

record-e2e:
	cd ui && npm run test:e2e:record -- $(if $(GREP),--grep "$(GREP)")

gen: gen-sqlc gen-api-client gen-docs

gen-sqlc:
	sqlc generate

gen-api-client:
	cd ui && npm run generate-api-client

gen-docs:
	cd ui && npm run generate-docs