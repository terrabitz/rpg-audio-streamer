FROM debian:trixie-slim AS ffmpeg-builder

RUN apt-get update && apt-get install -y --no-install-recommends \
  build-essential ca-certificates curl libmp3lame-dev libssl-dev nasm pkg-config xz-utils
COPY scripts/build-ffmpeg.sh /build-ffmpeg.sh
RUN sh /build-ffmpeg.sh /opt/ffmpeg

FROM node:24-trixie-slim AS node

FROM mcr.microsoft.com/devcontainers/go:1.26-trixie

USER root
RUN apt-get update && apt-get install -y --no-install-recommends \
  build-essential \
  ca-certificates \
  curl \
  git-lfs \
  libmp3lame0 \
  libssl3t64 \
  libstdc++6 \
  pkg-config \
  python3-venv \
  unzip \
  && rm -rf /var/lib/apt/lists

COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/npm
RUN ln -sf ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
  && ln -sf ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx

COPY --from=ffmpeg-builder /opt/ffmpeg /opt/ffmpeg
COPY scripts/install-libdave.sh /tmp/install-libdave.sh
RUN sh /tmp/install-libdave.sh /opt/libdave \
  && rm /tmp/install-libdave.sh \
  && printf '%s\n' /opt/ffmpeg/lib /opt/libdave/lib > /etc/ld.so.conf.d/rpg-audio-streamer.conf \
  && ldconfig

RUN python3 -m venv /opt/yt-dlp \
  && /opt/yt-dlp/bin/pip install --no-cache-dir 'yt-dlp[default]'

ARG TARGETARCH
RUN set -eu; \
  case "$TARGETARCH" in \
    amd64) \
      air_sha=d3b39beaee67ae1ccfa3a3a870be41d87fe89405109ef31114bfb0aadb9be4b9; \
      sqlc_sha=4bb811cb031bb04ed45e75e574346fdc5205e6bf2866a60a5da776b4eb306371 ;; \
    arm64) \
      air_sha=91b367e8e7b2479e96206f07c1cd6fa103e283b467067ceeb0b740937e79707b; \
      sqlc_sha=364f006097e849a4b7cc2e3d981cee46f6760104d216c562ca9f50ee175b682b ;; \
    *) echo "Unsupported development tool architecture: $TARGETARCH" >&2; exit 1 ;; \
  esac; \
  download_dir=$(mktemp -d); \
  cd "$download_dir"; \
  curl -fsSL --retry 3 "https://github.com/air-verse/air/releases/download/v1.67.4/air_1.67.4_linux_${TARGETARCH}" -o air; \
  curl -fsSL --retry 3 "https://github.com/sqlc-dev/sqlc/releases/download/v1.29.0/sqlc_1.29.0_linux_${TARGETARCH}.tar.gz" -o sqlc.tar.gz; \
  printf '%s  air\n%s  sqlc.tar.gz\n' "$air_sha" "$sqlc_sha" | sha256sum -c -; \
  tar -xzf sqlc.tar.gz sqlc; \
  install -m 0755 air sqlc /usr/local/bin/; \
  rm -f air sqlc sqlc.tar.gz; \
  cd /; \
  rmdir "$download_dir"

# Login shells reset PATH, so expose the media tools in a standard bin directory.
RUN ln -sf /opt/ffmpeg/bin/ffmpeg /opt/ffmpeg/bin/ffprobe /opt/yt-dlp/bin/yt-dlp /usr/local/bin/

ENV PKG_CONFIG_PATH="/opt/libdave/lib/pkgconfig" \
  CGO_ENABLED=1

# Some Docker DNS forwarders reject EDNS0 queries from the Go module client.
ENV GODEBUG=netedns0=0

ENV PLAYWRIGHT_BROWSERS_PATH=/opt/playwright
RUN --mount=type=bind,source=ui/package-lock.json,target=/tmp/ui-package-lock.json \
  version="$(node -p 'require("/tmp/ui-package-lock.json").packages["node_modules/playwright"].version')" \
  && npm exec --yes --cache /tmp/npm-cache --package "playwright@$version" -- playwright install --with-deps chromium \
  && rm -rf /tmp/npm-cache /var/lib/apt/lists/* \
  && chown -R vscode:vscode "$PLAYWRIGHT_BROWSERS_PATH"