# SPDX-License-Identifier: MIT
FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/licensify-server ./cmd/licensify-server

FROM alpine:3.20
RUN adduser -D -H -u 10001 licensify
USER licensify
WORKDIR /app
COPY --from=build /out/licensify-server /app/licensify-server
COPY migrations /app/migrations
EXPOSE 8080
ENTRYPOINT ["/app/licensify-server"]

