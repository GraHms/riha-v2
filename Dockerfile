# One image: the Go API serving the built frontend.
FROM node:22-alpine AS web
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ .
RUN npm run build

FROM golang:1.24-alpine AS api
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN go build -ldflags="-s -w" -o /out/riha ./cmd/server
RUN mkdir -p /out/data/uploads /out/data/outbox

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=api /out/riha /app/riha
COPY --from=web /app/dist /app/static
# A new named volume takes this owner, so the API can write to it.
COPY --from=api --chown=nonroot:nonroot /out/data /app/data
USER nonroot:nonroot
EXPOSE 8080
ENV RIHA_ENV=production RIHA_ADDR=:8080 RIHA_STATIC_DIR=/app/static RIHA_UPLOAD_DIR=/app/data/uploads RIHA_OUTBOX_DIR=/app/data/outbox
ENTRYPOINT ["/app/riha"]
