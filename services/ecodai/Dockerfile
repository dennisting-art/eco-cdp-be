# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY services/ecodai/go.mod services/ecodai/go.sum ./services/ecodai/
WORKDIR /src/services/ecodai
RUN go mod download
COPY pkg /src/pkg
COPY services/ecodai /src/services/ecodai
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ecodai ./cmd/server

FROM migrate/migrate:v4.19.1 AS migrate
FROM alpine:3.21

RUN apk upgrade --no-cache \
	&& apk add --no-cache ca-certificates \
	&& addgroup -S ecodai \
	&& adduser -S -G ecodai ecodai

COPY --from=build /out/ecodai /app/ecodai
COPY --from=migrate /usr/local/bin/migrate /usr/local/bin/migrate
COPY services/ecodai/migrations /migrations
COPY services/ecodai/docs/openapi.yaml /app/docs/openapi.yaml
COPY services/ecodai/docs/swagger-ui/index.html /app/docs/swagger-ui/index.html

USER ecodai
EXPOSE 8080
ENTRYPOINT ["/app/ecodai"]
