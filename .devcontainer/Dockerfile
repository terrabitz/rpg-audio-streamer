FROM mcr.microsoft.com/devcontainers/go:1.25

ARG NODE_VERSION=24
RUN . $NVM_DIR/nvm.sh \
  && nvm install ${NODE_VERSION} \
  && nvm use ${NODE_VERSION} \
  && nvm alias default ${NODE_VERSION}

# Install ffmpeg
RUN apt-get update \
  && apt install -y \
  ffmpeg \
  git-lfs \
  && go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
