FROM mcr.microsoft.com/devcontainers/base:ubuntu

# Install Bun system-wide so all container users can access it
ENV BUN_INSTALL=/usr/local
RUN curl -fsSL https://bun.sh/install | bash

# Install Playwright system dependencies (Chromium)
RUN bunx --yes playwright install --with-deps chromium

# Install dependencies on mount
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile
