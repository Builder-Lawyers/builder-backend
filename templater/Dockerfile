FROM golang:1.24.5 AS build
WORKDIR /app
COPY /templater /app
COPY templater/go.mod templater/go.sum /modules/

RUN go mod download
RUN go env -w CGO_ENABLED=0

RUN go build -o main ./main.go

FROM ubuntu:20.04
RUN apt-get update && apt-get install -y curl
COPY --from=build /app/main /usr/local/bin/