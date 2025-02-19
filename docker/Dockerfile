# Stage 1: Build Vue frontend
FROM node:20 AS frontend-builder

WORKDIR /ui
COPY ui .
RUN npm install && npm run build

# Stage 2: Build Go backend
FROM golang:1.24-alpine AS backend-builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY main.go .
COPY sql sql
COPY internal internal

RUN CGOENABLED=1 go build -o backend main.go

# Stage 3: Create the final runtime image
FROM alpine:3.21.3

ARG TARGETARCH

# Add s6-overlay with architecture support
RUN S6_ARCH=$(case ${TARGETARCH} in \
  "amd64") echo "x86_64";; \
  "arm64") echo "aarch64";; \
  *) echo "unsupported architecture: ${TARGETARCH}" && exit 1;; \
  esac) && \
  wget -q "https://github.com/just-containers/s6-overlay/releases/download/v3.1.6.2/s6-overlay-noarch.tar.xz" -O /tmp/s6-overlay-noarch.tar.xz && \
  wget -q "https://github.com/just-containers/s6-overlay/releases/download/v3.1.6.2/s6-overlay-${S6_ARCH}.tar.xz" -O /tmp/s6-overlay-arch.tar.xz && \
  tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz && \
  tar -C / -Jxpf /tmp/s6-overlay-arch.tar.xz && \
  rm /tmp/s6-overlay-*.tar.xz

# Install OS packages
RUN apk add --no-cache \
  ffmpeg \
  caddy

# Set up directories
WORKDIR /app

# Copy built backend and frontend
COPY --from=backend-builder /app/backend /app/backend
COPY --from=frontend-builder /ui/dist /app/frontend

# Copy Caddy and s6 configurations
COPY docker/Caddyfile /etc/caddy/Caddyfile
COPY docker/s6-rc.d /etc/s6-overlay/s6-rc.d

# Create directories and set permissions
RUN mkdir -p /data/app/uploads \
  && mkdir -p /config \
  && chown -R nobody:nobody /data \
  && chown -R nobody:nobody /app \
  && chown -R nobody:nobody /config

# Set default env vars for Docker layout
ENV DB_PATH=/data/app/skaldbot.db \
  UPLOAD_DIR=/data/app/uploads

# Set s6-overlay environment variables
ENV  S6_KEEP_ENV=1 \
  S6_BEHAVIOUR_IF_STAGE2_FAILS=2 \
  S6_CMD_WAIT_FOR_SERVICES_MAXTIME=0

# Set s6-overlay environment variables
ENV  S6_KEEP_ENV=1 \
  S6_BEHAVIOUR_IF_STAGE2_FAILS=2 \
  S6_CMD_WAIT_FOR_SERVICES_MAXTIME=0

# Expose ports
EXPOSE 80 443

ENTRYPOINT ["/init"]
