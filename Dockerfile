# Stage 1: Build Go API binary
FROM golang:1.25-alpine AS go-builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /api ./cmd/main.go

# Stage 2: Build React frontend
FROM node:22-alpine AS web-builder

WORKDIR /app

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ .
RUN npm run build

# Stage 3: Final runtime image
FROM node:22-alpine

RUN apk add --no-cache tini

WORKDIR /app

# Copy Go API binary
COPY --from=go-builder /api /api

# Copy React build output and server dependencies
COPY --from=web-builder /app/build/ /app/build/
COPY --from=web-builder /app/package.json /app/package.json
COPY --from=web-builder /app/node_modules/ /app/node_modules/

EXPOSE 3000 8080

ENTRYPOINT ["/sbin/tini", "--"]
CMD ["sh", "-c", "/api & npx react-router-serve ./build/server/index.js"]
