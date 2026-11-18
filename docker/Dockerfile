ARG FFMPEG_VARIANT=debian

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

RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o backend main.go \
  && strip --strip-unneeded /opt/libdave/lib/libdave.so

# Stage 3: Install the downloader without runtime installation tooling
FROM debian:trixie-slim AS youtube-builder

RUN apt-get update && apt-get install -y --no-install-recommends python3-venv \
  && python3 -m venv /opt/installer \
  && python3 -m venv --without-pip /opt/yt-dlp \
  && /opt/installer/bin/pip --python /opt/yt-dlp/bin/python install \
    --no-cache-dir --no-compile 'yt-dlp[default]'

# Build the opt-in headless FFmpeg variant from Debian's signed source packages.
FROM debian:trixie-slim AS ffmpeg-builder

RUN sed -i 's/^Types: deb$/Types: deb deb-src/' /etc/apt/sources.list.d/debian.sources \
  && apt-get update \
  && apt-get install -y --no-install-recommends \
    build-essential libmp3lame-dev libssl-dev nasm pkg-config \
  && mkdir /src \
  && cd /src \
  && apt-get source ffmpeg

RUN cd /src/ffmpeg-* \
  && ./configure --prefix=/opt/ffmpeg \
    --enable-shared --disable-static \
    --disable-autodetect --disable-doc --disable-debug \
    --disable-devices --enable-indev=lavfi \
    --disable-ffplay --enable-version3 --enable-openssl --enable-libmp3lame \
  && make -j2 ffmpeg ffprobe \
  && make install \
  && rm -rf /opt/ffmpeg/lib/pkgconfig \
  && mkdir -p /opt/ffmpeg/licenses \
  && cp COPYING.* LICENSE.md /opt/ffmpeg/licenses/

# The default image retains Debian's complete FFmpeg feature set.
FROM debian:trixie-slim AS ffmpeg-debian

RUN apt-get update && apt-get install -y --no-install-recommends \
  ca-certificates \
  ffmpeg \
  libstdc++6 \
  nodejs \
  python3 \
  && rm -rf /var/lib/apt/lists

# Experimental: native codecs and HTTPS, without desktop or hardware integrations.
FROM debian:trixie-slim AS ffmpeg-minimal

RUN apt-get update && apt-get install -y --no-install-recommends \
  ca-certificates \
  libmp3lame0 \
  libssl3t64 \
  libstdc++6 \
  nodejs \
  python3 \
  && rm -rf /var/lib/apt/lists
COPY --from=ffmpeg-builder /opt/ffmpeg/bin/ /usr/local/bin/
COPY --from=ffmpeg-builder /opt/ffmpeg/lib/ /usr/local/lib/
COPY --from=ffmpeg-builder /opt/ffmpeg/licenses/ /usr/share/licenses/ffmpeg/

FROM ffmpeg-${FFMPEG_VARIANT} AS runtime

COPY --from=youtube-builder /opt/yt-dlp /opt/yt-dlp
ENV PATH="/opt/yt-dlp/bin:${PATH}" \
  PYTHONDONTWRITEBYTECODE=1

# Set up directories
WORKDIR /app

# Copy built backend and frontend
COPY --from=backend-builder --chown=nobody:nogroup /app/backend /app/backend
COPY --from=frontend-builder --chown=nobody:nogroup /ui/dist /app/frontend
COPY --from=backend-builder /opt/libdave/lib/libdave.so /usr/local/lib/libdave.so
COPY --from=backend-builder /opt/libdave/share/licenses/libdave /usr/share/licenses/libdave
RUN ldconfig

# Create directories and set permissions
RUN mkdir -p /app/data/uploads \
  && chown nobody:nogroup /app /app/data /app/data/uploads

# Set default env vars for Docker layout
ENV DB_PATH=/app/data/skaldbot.db \
  UPLOAD_DIR=/app/data/uploads \
  FRONTEND_DIR=/app/frontend \
  PORT=80

# Expose ports
EXPOSE 80

ENTRYPOINT ["/app/backend"]
CMD ["serve"]
