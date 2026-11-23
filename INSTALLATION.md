
# Installing rpg-audio-stramer

## Option 1: Docker Compose (Recommended)

1. Create a `docker-compose.yml`:

   ```yaml
   # examples/docker-compose/docker-compose.yml
   
   services:
     rpg-audio-streamer:
       image: ghcr.io/terrabitz/rpg-audio-streamer:latest
       restart: unless-stopped
       ports:
         - "8080:80"
       volumes:
         - app_data:/app/data
   
   volumes:
     app_data:
   
   ```

2. Start the server and get the setup code from the logs:

   ```bash
   # examples/docker-compose/install.sh
   
   # Start the server
   docker compose up -d
   
   # Get the setup code from the logs
   docker compose logs rpg-audio-streamer
   
   ```

3. Use the setup code to log in at http://localhost:8080

## Option 2: Plain Docker

1. Start the server and get the setup code from the logs:

   ```bash
   # examples/docker/install.sh
   
   # Create the data directory
   mkdir -p data/uploads
   
   # Run the container
   docker run -d \
     --name rpg-audio-streamer \
     -p 8080:80 \
     -v $PWD/data:/app/data \
     ghcr.io/terrabitz/rpg-audio-streamer:latest
   
   # Get the setup code from the logs
   docker logs rpg-audio-streamer
   
   ```

2. Use the setup code to log in at http://localhost:8080

## Configuration

| Environment variable | Flag | CLI default | Description |
| --- | --- | --- | --- |
| `DISCORD_STREAMING_BOT_TOKEN` | `--discord-streaming-bot-token` | Unset | Bot token for Discord voice streaming; prefer the environment variable |
| `DISCORD_STREAMING_ENABLED` | `--discord-streaming-enabled` | `false` | Enable Discord voice streaming |
| `HOST` | `--host` | Unset | Host to bind (empty listens on all interfaces) |
| `PORT` | `--port` | `8080` | Port to listen on |
| `CORS_ORIGINS` | `--cors-origins` | Unset | Allowed CORS origins |
| `LOG_FORMAT` | `--log-format` | `"json"` | Log format (json or pretty) |
| `LOG_LEVEL` | `--log-level` | `"info"` | Log level (debug, info, warn, error) |
| `UPLOAD_DIR` | `--upload-dir` | `"./uploads"` | Directory to store uploaded files |
| `FRONTEND_DIR` | `--frontend-dir` | Unset | Directory containing the built SPA (empty disables frontend serving) |
| `ROOT_USERNAME` | `--root-username` | `"admin"` | Initial admin username |
| `ROOT_PASSWORD` | `--root-password` | Unset | Initial admin password (used only when no admin exists) |
| `ROOT_PASSWORD_HASH` | `--root-password-hash` | Unset | Initial admin Argon2id password hash (alternative to ROOT_PASSWORD) |
| `TOKEN_SECRET` | `--token-secret` | Unset | Secret for signing JWT tokens |
| `TOKEN_DURATION` | `--token-duration` | `24h0m0s` | Duration for JWT tokens |
| `TOKEN_ISSUER` | `--token-issuer` | `"rpg-audio-streamer"` | Issuer for JWT tokens |
| `TOKEN_AUDIENCE` | `--token-audience` | `"rpg-audio-streamer-client"` | Audience for JWT tokens |
| `DEV_MODE` | `--dev-mode` | `false` | Enables development mode |
| `DB_PATH` | `--db-path` | `"skaldbot.db"` | Path to SQLite database file |