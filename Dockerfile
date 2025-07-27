# Reference: https://hub.docker.com/_/golang/tags?name=1.24-alpine
ARG GOLANG_VERSION=1.24
ARG ALPINE_VERSION=3.22

FROM golang:${GOLANG_VERSION}-alpine${ALPINE_VERSION} AS build_deps

RUN apk add --no-cache git

WORKDIR /workspace
ENV GO111MODULE=on

COPY go.mod .
COPY go.sum .

RUN go mod download

FROM build_deps AS build

COPY . .

RUN CGO_ENABLED=0 go build -o oci-dyndns -ldflags '-w -extldflags "-static"' cmd/main.go

FROM alpine:${ALPINE_VERSION}
ARG UID=1000
ARG GID=1000

RUN adduser -u ${UID} -g ${GID} app -D
COPY --from=build /workspace/oci-dyndns /usr/local/bin/oci-dyndns
USER ${UID}

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/oci-dyndns", "--config", "/config.json"]
