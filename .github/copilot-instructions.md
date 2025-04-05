# Backend Instructions

When interacting with the database, do not write any code directly in the `sqlitedb` folder. Do not write any Go code that directly executes SQL. Instead, write all queries in the `sql/queries` folder using plain SQL queries. I will then use sqlc to covnert that to Go code in `sqlitedb`. Once that code is generated, we can then hook it up to our domain types using the code in the `sqlitedatastore` folder.

# Frontend Instructions

Do not write any API client code under the `apiClient` folder. Instead, endpoints should be added to the `openapi.yaml` spec. Then I will run the `npm run generate-api-client` utility to turn this into TS client code.