.PHONY: ui sqlc go test

sqlc:
	sqlc generate

ui:
	cd ui && npm install && npm run dev

go:
	air

test:
	go test -v -cover ./...

generate:
	cd ui && npm run generate-api-client