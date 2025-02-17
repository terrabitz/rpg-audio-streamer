# RPG Audio Streamer

A simple synchronized soundboard designed for tabletop RPGs, allowing game masters to manage and stream background music and sound effects during gameplay sessions.

![RPG soundboard](./media/audio-playing.gif)

## Features

- 🎛️ Soundboard for many kinds of RPG audio sounds (ambiance, music, one-shot sound effects)
- 🌐 Real-time synchronized streaming to players
- 🎚️ Fading for smooth transitions between audio tracks
- 🎼 Automatic re-encoding for efficient streaming

## Installation

### Prerequisites

- Go 1.21 or later
- FFmpeg
- Node.js 20+ (for UI development)

### Quick Start

1. Clone the repository:
   ```bash
   git clone https://github.com/terrabitz/rpg-audio-streamer.git
   cd rpg-audio-streamer
   ```

2. Build the UI:
   ```bash
   cd ui
   npm install
   npm run build
   cd ..
   ```

3. Set up environment (choose one method):

   ```bash
   # 1. Generate password hash
   go run scripts/hash_password.go

   # 2. Generate secure tokens
   TOKEN_SECRET=$(openssl rand -hex 64)
   JOIN_TOKEN=$(openssl rand -hex 32)

   # 3. Create and edit .env file
   cp .env.example .env
   ```

   Then edit `.env` with your settings:
   ```env
   ROOT_USERNAME=<your username>
   ROOT_PASSWORD_HASH=<hash from step 1>
   TOKEN_SECRET=<token from step 2>
   JOIN_TOKEN=<join token from step 2>
   ```

4. Build and run:
   ```bash
   go build
   ./rpg-audio-streamer serve
   ```

## Configuration

The server can be configured using environment variables or command-line flags:

### Server Options
- `PORT` (default: 8080) - Server listening port
- `CORS_ORIGINS` - Allowed CORS origins
- `UPLOAD_DIR` (default: ./uploads) - Directory for audio files
- `DEV_MODE` - Enable development features

### Authentication
- `ROOT_USERNAME` (default: admin) - Admin username
- `ROOT_PASSWORD_HASH` (required) - Argon2id hash of admin password
- `TOKEN_SECRET` (required) - JWT signing secret
- `TOKEN_DURATION` (default: 24h) - JWT token validity duration
- `JOIN_TOKEN` - Static token for player authentication

### Logging
- `LOG_FORMAT` (default: json) - Log format (json/pretty)
- `LOG_LEVEL` (default: info) - Log level (debug/info/warn/error)

## API Endpoints

### Authentication
- `POST /api/v1/auth/login` - Admin login
- `POST /api/v1/auth/join` - Player join with token

### File Management
- `GET /api/v1/files` - List available audio files
- `POST /api/v1/files` - Upload new audio files
- `DELETE /api/v1/files/{filename}` - Delete an audio file
- `GET /api/v1/stream/{filename}` - Stream an audio file

## Developer Guide

### Project Structure

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

### Development Setup

1. Install [air](https://github.com/air-verse/air) if you haven't done so already

   ```bash
   go install github.com/air-verse/air@latest
   ```

2. Start the backend:

   ```bash
   air
   ```

3. In a separate termina, start the Vue dev server:

   ```bash
   cd ui
   npm run dev
   ```

### Database Migrations

Manage database schema:
```bash
# Apply all migrations
./rpg-audio-streamer migrate up

# Revert last migration
./rpg-audio-streamer migrate down

# Check current version
./rpg-audio-streamer migrate version
```

### Testing

```bash
# Run all tests
go test ./...

# Run UI tests
cd ui && npm test
```

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

MIT License - See LICENSE file for details

