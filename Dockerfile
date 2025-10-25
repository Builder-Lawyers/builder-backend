FROM golang:1.24.5 AS build
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -o /bin/main ./main.go

FROM node:20-alpine
RUN npm install -g pnpm
COPY --from=build /bin/main /usr/local/bin/main
COPY api /api
ENTRYPOINT ["main"]