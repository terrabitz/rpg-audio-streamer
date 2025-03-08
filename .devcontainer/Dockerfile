FROM mcr.microsoft.com/devcontainers/go:1-1.23-bookworm

ARG NODE_VERSION=20
RUN nvm install ${NODE_VERSION} && nvm use ${NODE_VERSION} && nvm alias default ${NODE_VERSION}

# Install ffmpeg
RUN apt-get install -y ffmpeg