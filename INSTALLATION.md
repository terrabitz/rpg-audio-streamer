
# Installing rpg-audio-stramer

## Option 1: Docker Compose (Recommended)


1. Create a docker-compose.yml:

   ```yaml
   services:
     rpg-audio-streamer:
       image: ghcr.io/terrabitz/rpg-audio-streamer:latest
       restart: unless-stopped
       ports:
         - "8080:80"
       volumes:
         - appdata:/app/data

   volumes:
     app_data:
   ```

2. Start the server:

   ```bash
   docker compose up -d
   ```
   
3. Grab the claim code from the logs

   ```bash
   docker compose logs rpg-audio-streamer
   ```

## Option 2: Plain Docker

1. Create data directories:

   ```bash
   mkdir -p data/uploads
   ```

2. Run the container:

   ```bash
   docker run -d \
     --name rpg-audio-streamer \
     -p 8080:80 \
     -v $PWD/data:/app/data \
     ghcr.io/terrabitz/rpg-audio-streamer:latest
   ```

3. Use the setup code in the logs to login at http://localhost:8080

   ```bash
   docker logs rpg-audio-streamer
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
