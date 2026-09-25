# Stage 1: Build Go API binary
FROM golang:1.25-alpine AS go-builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /api ./cmd/main.go

# Stage 2: Build React frontend (same Node major as the runtime and CI)
FROM node:24-alpine AS web-builder

WORKDIR /app

COPY web/package.json web/package-lock.json web/.npmrc ./
RUN --mount=type=secret,id=npm_token \
    echo "//npm.pkg.github.com/:_authToken=$(cat /run/secrets/npm_token)" >> .npmrc && \
    npm ci && \
    sed -i '/_authToken/d' .npmrc

COPY web/ .
RUN npm run build

# Stage 3: Final runtime image
FROM node:24-alpine

LABEL org.opencontainers.image.title="cluster-vision" \
      org.opencontainers.image.source="https://github.com/fredericrous/cluster-vision" \
      org.opencontainers.image.licenses="LicenseRef-BUSL-1.1" \
      org.opencontainers.image.vendor="Frederic Rous"

RUN apk add --no-cache tini

WORKDIR /app

COPY --chmod=0755 docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

# Copy Go API binary
COPY --from=go-builder /api /api

# Copy React build output and server dependencies
COPY --from=web-builder /app/build/ /app/build/
COPY --from=web-builder /app/package.json /app/package.json
COPY --from=web-builder /app/node_modules/ /app/node_modules/

EXPOSE 3000 8080

# The base image's unprivileged user (uid/gid 1000). Neither server writes
# outside /tmp, so the image also runs with a read-only root filesystem.
USER node

# tini -g delivers SIGTERM to the whole process group, so both servers shut
# down gracefully; the entrypoint stops the pair when either one dies.
# Arguments are flags for the Go API, e.g. `-port=8080 -refresh=5m`.
ENTRYPOINT ["/sbin/tini", "-g", "--", "/usr/local/bin/docker-entrypoint.sh"]
CMD []
