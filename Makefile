.PHONY: ui go test gen gen-sqlc gen-api-client gen-docs


ui:
	cd ui && npm install && npm run dev

go:
	air

test:
	go test -v -cover ./...

gen: gen-sqlc gen-api-client gen-docs

gen-sqlc:
	sqlc generate

gen-api-client:
	cd ui && npm run generate-api-client

gen-docs:
	cd ui && npm run generate-docs