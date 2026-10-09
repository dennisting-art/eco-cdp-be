# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/eco-cdp-be ./services/eco/cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && addgroup -S eco && adduser -S -G eco eco
COPY --from=build /out/eco-cdp-be /app/eco-cdp-be
USER eco
EXPOSE 8080
ENTRYPOINT ["/app/eco-cdp-be"]
