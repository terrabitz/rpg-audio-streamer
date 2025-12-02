# RPG Audio Streamer

A simple synchronized soundboard designed for tabletop RPGs, allowing game masters to manage and stream background music and sound effects during gameplay sessions.

![RPG soundboard](./media/audio-playing.gif)

## Features

- 🎛️ Soundboard for many kinds of RPG audio sounds (ambiance, music, one-shot sound effects)
- 🌐 Real-time synchronized streaming to players
- 🎚️ Fading for smooth transitions between audio tracks
- 🎼 Automatic re-encoding for efficient streaming

## Installation

### Using Docker (Recommended)

#### Option 1: Docker Compose

1. Create a new directory and add the following files:

   ```bash
   mkdir rpg-audio-streamer && cd rpg-audio-streamer
   ```

<!-- FIXME: fix this step -->
2. Generate a password hash:
   ```bash
   go run ./cmd/hashpass/main.go
   ```

3. Create an environment file:
   ```bash
   # Generate secure tokens
   TOKEN_SECRET=$(openssl rand -hex 64)
   JOIN_TOKEN=$(openssl rand -hex 32)

   # Create .env file
   cat > .env << EOL
   ROOT_USERNAME=admin
   ROOT_PASSWORD_HASH='<password-from-step-2>'
   TOKEN_SECRET=$TOKEN_SECRET
   JOIN_TOKEN=$JOIN_TOKEN
   HOSTNAME=<your-server-hostname>
   EOL
   ```

4. Create a docker-compose.yml:
   ```yaml
   services:
     app:
       image: ghcr.io/terrabitz/rpg-audio-streamer:latest
       restart: unless-stopped
       env_file: .env
       ports:
         - "80:80"
         - "443:443"
       volumes:
         - data:/data/app
         - caddy_data:/data/caddy
         - caddy_config:/config/caddy

   volumes:
     data:
     caddy_data:
     caddy_config:
   ```

5. Start the server:
   ```bash
   docker compose up -d
   ```

#### Option 2: Plain Docker

1. Create data directories:
   ```bash
   mkdir -p data/app/uploads
   ```

2. Generate configuration:
   ```bash
   # Generate secure tokens
   TOKEN_SECRET=$(openssl rand -hex 64)
   JOIN_TOKEN=$(openssl rand -hex 32)

   # Generate password hash
   docker run --rm ghcr.io/terrabitz/rpg-audio-streamer:latest ./server hash-password
   ```

3. Run the container:
   ```bash
   docker run -d \
     --name rpg-audio-streamer \
     -p 80:80 \
     -e ROOT_USERNAME=admin \
     -e ROOT_PASSWORD_HASH=<hash from step 2> \
     -e TOKEN_SECRET=$TOKEN_SECRET \
     -e JOIN_TOKEN=$JOIN_TOKEN \
     -e UPLOAD_DIR=/data/app/uploads \
     -e DB_PATH=/data/app/skaldbot.db \
     -v $PWD/data:/data/app \
     ghcr.io/terrabitz/rpg-audio-streamer:latest
   ```

### Manual Installation (Alternative)

#### Prerequisites

- Go 1.21 or later
- FFmpeg
- Node.js 20+ (for UI development)

#### Quick Start

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

