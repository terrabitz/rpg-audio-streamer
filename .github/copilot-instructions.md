# General Instructions

When writing code, always follow the project's coding standards and best practices. Ensure that your code is well-documented, follows the existing code style, and includes appropriate error handling.

Use minimal comments. Don't write comments that explain what the code does; instead, focus on why the code is written that way. Use meaningful variable and function names to make the code self-explanatory. Avoid writing overly complex code; strive for simplicity and clarity. If a piece of code is particularly complex, consider breaking it down into smaller, more manageable functions.

Stick with the Go design practice of "happy path on the left" when writing functions. This means that the main logic of the function should be on the left side, with error handling and edge cases on the right side. This makes it easier to read and understand the primary flow of the function.

Do not use HEREDOCS for writing content to a file.

# Backend Instructions

When interacting with the database, do not write any code directly in the `sqlitedb` folder. Do not write any Go code that directly executes SQL. Instead, write all queries in the `sql/queries` folder using plain SQL queries. Then run the `make gen-sqlc` command to generate the Go code that interacts with the database. This will ensure that the database interactions are always in sync with the SQL queries.

# Frontend Instructions

Do not write any API client code under the `apiClient` folder. Instead, endpoints should be added to the `openapi.yaml` spec. Then run the `make gen-api-client` command to generate the API client code. This will ensure that the API client is always in sync with the backend API.