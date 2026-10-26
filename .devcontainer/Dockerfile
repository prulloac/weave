FROM mcr.microsoft.com/devcontainers/base:ubuntu

# Bun is used only for Markdown linting.
ENV BUN_INSTALL=/usr/local
RUN curl -fsSL https://bun.sh/install | bash

# Install docs tooling.
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile
