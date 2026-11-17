# Stage 1: Build Vue frontend
FROM node:24 AS frontend-builder

WORKDIR /ui
COPY ui .
RUN npm ci && npm run build

# Stage 2: Build Go backend
FROM golang:1.26-trixie AS backend-builder

WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends curl unzip pkg-config
COPY scripts/install-libdave.sh scripts/install-libdave.sh
RUN sh scripts/install-libdave.sh /opt/libdave
ENV PKG_CONFIG_PATH=/opt/libdave/lib/pkgconfig
COPY go.mod go.sum ./
RUN go mod download

COPY main.go .
COPY sql sql
COPY internal internal

RUN CGOENABLED=1 go build -o backend main.go

# Stage 3: Create the final runtime image
FROM debian:trixie-slim

# Install OS packages
RUN apt-get update && apt-get install -y --no-install-recommends \
  ca-certificates \
  ffmpeg \
  libstdc++6 \
  nodejs \
  python3-venv \
  && python3 -m venv /opt/yt-dlp \
  && /opt/yt-dlp/bin/pip install --no-cache-dir 'yt-dlp[default]'

ENV PATH="/opt/yt-dlp/bin:${PATH}"

# Set up directories
WORKDIR /app

# Copy built backend and frontend
COPY --from=backend-builder /app/backend /app/backend
COPY --from=frontend-builder /ui/dist /app/frontend
COPY --from=backend-builder /opt/libdave/lib/libdave.so /usr/local/lib/libdave.so
COPY --from=backend-builder /opt/libdave/share/licenses/libdave /usr/share/licenses/libdave
RUN ldconfig

# Create directories and set permissions
RUN mkdir -p /app/data/uploads \
  && chown -R nobody:nogroup /app

# Set default env vars for Docker layout
ENV DB_PATH=/app/data/skaldbot.db \
  UPLOAD_DIR=/app/data/uploads \
  FRONTEND_DIR=/app/frontend \
  PORT=80

# Expose ports
EXPOSE 80

ENTRYPOINT ["/app/backend"]
CMD ["serve"]
