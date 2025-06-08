# Backend Instructions

When interacting with the database, do not write any code directly in the `sqlitedb` folder. Do not write any Go code that directly executes SQL. Instead, write all queries in the `sql/queries` folder using plain SQL queries. Then run the `make gen-sqlc` command to generate the Go code that interacts with the database. This will ensure that the database interactions are always in sync with the SQL queries.

# Frontend Instructions

Do not write any API client code under the `apiClient` folder. Instead, endpoints should be added to the `openapi.yaml` spec. Then run the `make gen-api-client` command to generate the API client code. This will ensure that the API client is always in sync with the backend API.