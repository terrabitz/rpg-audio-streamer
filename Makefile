.PHONY: ui sqlc go

sqlc:
	sqlc generate

ui:
	cd ui && npm install && npm run dev

go:
	air
