# Stage 1: Build Vue frontend
FROM node:20 AS frontend-builder

WORKDIR /ui
COPY ui .
RUN npm install && npm run build

# Stage 2: Build Go backend
FROM golang:1.24 AS backend-builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY main.go .
COPY sql sql
COPY internal internal

RUN CGOENABLED=1 go build -tags netgo,osusergo -ldflags "-linkmode 'external' -extldflags '-static'" -o server main.go

# Stage 3: Create the final runtime image
FROM caddy:2.7-alpine

# Install FFmpeg and supervisord
RUN apk add --no-cache ffmpeg supervisor

# Set up directories
WORKDIR /app

# Copy built backend and frontend
COPY --from=backend-builder /app/server /app/server
COPY --from=frontend-builder /ui/dist /srv

# Copy Caddy and supervisor configurations
COPY docker/Caddyfile /etc/caddy/Caddyfile
COPY docker/supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# Expose HTTP and HTTPS ports
EXPOSE 80 443 8080

RUN mkdir -p /data/app/uploads  \
  && mkdir -p /config \
  && chown -R nobody:nobody /data  \
  && chown -R nobody:nobody /app  \
  && chown -R nobody:nobody /config  \
  && chown -R nobody:nobody /srv

# Set default env vars for Docker layout
ENV DB_PATH=/data/app/skaldbot.db \
  UPLOAD_DIR=/data/app/uploads

# Start supervisord
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
