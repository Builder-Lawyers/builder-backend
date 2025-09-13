FROM golang:1.24.5 AS build
WORKDIR /app
COPY /api /app/api
COPY "/cmd" "/app/cmd"
COPY /pkg /app/pkg
COPY /internal /app/internal
COPY ./main.go /app
COPY ./go.mod ./go.sum /app/

RUN go mod download
RUN go env -w CGO_ENABLED=0

RUN go build -o main ./main.go

FROM ubuntu:20.04
RUN apt-get update && apt-get install -y \
    curl \
    ca-certificates \
    gnupg \
    git \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && npm install -g pnpm

RUN node -v && npm -v && pnpm -v
COPY --from=build /app/main /usr/local/bin/
COPY --from=build /app/api /api