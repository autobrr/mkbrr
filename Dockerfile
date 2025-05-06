# build app
FROM golang:1.24-alpine3.21 AS app-builder

ARG VERSION=dev
ARG REVISION=dev
ARG BUILDTIME

RUN apk add --no-cache git build-base tzdata

ENV SERVICE=mkbrr

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

#ENV GOOS=linux
#ENV CGO_ENABLED=0

RUN go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${REVISION} -X main.date=${BUILDTIME}" -o bin/mkbrr main.go

# build runner
FROM alpine:latest

LABEL org.opencontainers.image.source="https://github.com/autobrr/mkbrr"

ENV HOME="/config" \
    XDG_CONFIG_HOME="/config" \
    XDG_DATA_HOME="/config"

RUN apk --no-cache add ca-certificates

WORKDIR /app

VOLUME /config

COPY --from=app-builder /src/bin/mkbrr /usr/local/bin/

CMD ["/usr/local/bin/mkbrr"]