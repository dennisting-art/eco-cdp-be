# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ecodai ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && addgroup -S ecodai && adduser -S -G ecodai ecodai
COPY --from=build /out/ecodai /app/ecodai
USER ecodai
EXPOSE 8080
ENTRYPOINT ["/app/ecodai"]
