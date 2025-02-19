# RPG Audio Streamer Docker Image

Multi-arch Docker image for RPG Audio Streamer with integrated Caddy server and s6-overlay process supervision.

## Directory Layout

```
/
├── app/                    # Application directory
│   ├── backend            # Go backend binary
│   ├── frontend/          # Vue.js frontend static files
│   └── data/              # Application data
│       ├── uploads/       # Audio file storage
│       └── skaldbot.db    # SQLite database
├── caddy/                 # Caddy server files
│   └── data/             # TLS certificates and runtime data
└── etc/
    ├── caddy/            # Static Caddy configuration
    └── s6-overlay/       # Service definitions
```

## Volumes

Mount these volumes for persistence:
- `/app/data`: Application data (database and uploads)
- `/caddy/data`: Caddy data (certificates, config, and state)

## Ports

- 80: HTTP
- 443: HTTPS

## Process Management

Uses s6-overlay to manage:
- Caddy web server
- Go backend server

Services are configured to:
1. Start backend first
2. Start Caddy after backend is ready
3. Automatically restart on failure

## Examples

### Docker Compose
```yaml
services:
  app:
    image: ghcr.io/terrabitz/rpg-audio-streamer:latest
    volumes:
      - app_data:/app/data
      - caddy_data:/caddy/data
    ports:
      - "80:80"
      - "443:443"
    env_file: .env

volumes:
  app_data:
  caddy_data:
```

### Docker CLI
```bash
docker run -d \
  --name rpg-audio-streamer \
  -p 80:80 \
  -p 443:443 \
  -v app_data:/app/data \
  -v caddy_data:/caddy/data \
  --env-file .env \
  ghcr.io/terrabitz/rpg-audio-streamer:latest
```
