FROM golang:1.27.2-alpine AS builder
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /api ./cmd/api

FROM alpine:3.23
RUN addgroup -S app && adduser -S app -G app
COPY --from=builder /api /api
USER app
EXPOSE 8080
ENTRYPOINT ["/api"]
