FROM golang:1.18.3-alpine3.16 AS build
RUN apk --no-cache add build-base
WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go build -o app

FROM alpine:3.16
RUN apk --no-cache add tzdata ca-certificates && rm -rf /var/cache/apk/*
COPY --from=build /src/app /usr/local/bin/github-releases-notifier
ENTRYPOINT github-releases-notifier
