
# Developer Guide

## Project Structure

```
rpg-audio-streamer/
├── cmd/                 # Command-line entrypoints and helper tools
├── internal/
│   ├── auth/           # Authentication logic
│   ├── server/         # HTTP server implementation
│   ├── sqlitedatastore/# Database operations
│   └── websocket/      # WebSocket server
├── sql/
│   ├── migrations/     # Database migrations
│   └── queries/        # Database queries
├── ui/                 # Frontend Vue application
```

## Development Setup

1. Install [air](https://github.com/air-verse/air) if you haven't done so already

   ```bash
   go install github.com/air-verse/air@latest
   ```

2. Start the backend:

   ```bash
   air
   ```

3. In a separate terminal, start the Vue dev server:

   ```bash
   cd ui
   npm run dev
   ```

## Database Migrations

Manage database schema:
```bash
# Apply all migrations
./rpg-audio-streamer migrate up

# Revert last migration
./rpg-audio-streamer migrate down

# Check current version
./rpg-audio-streamer migrate version
```

## Testing

```bash
# Run all tests
go test ./...

# Run UI tests
cd ui && npm test
```

## Contributing Flow

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request