FROM golang:1.12.1-alpine3.9 AS build

RUN apk update && apk add ca-certificates && apk add git

WORKDIR /app

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build

FROM alpine:3.9

RUN apk update && apk add ca-certificates
WORKDIR /app
COPY --from=build /app/cluster-monitor-poc cluster-monitor-poc 
