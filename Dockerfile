# Reference: https://hub.docker.com/_/golang/tags?name=1.26-alpine
#
# Both images are named inline rather than through an ARG: Dependabot cannot resolve a version behind an ARG
# directive, so an ARG-based tag is one nothing watches (dependabot-core#2057, #4597, #10190).
#
# The build image uses the bare `-alpine` tag on purpose. The suffixed `-alpine<version>` variants stop being
# rebuilt once the Go team's default Alpine moves on, which silently freezes the toolchain at whatever patch was
# current that day -- `golang:1.25-alpine3.22` last changed 2026-06-03, which is how :main shipped on go1.25.11
# while 1.25.14 was out. The runtime base below stays pinned; only the builder needs to track upstream.
FROM golang:1.26-alpine AS build_deps

RUN apk add --no-cache git

WORKDIR /workspace
ENV GO111MODULE=on

COPY go.mod .
COPY go.sum .

RUN go mod download

FROM build_deps AS build

COPY . .

RUN CGO_ENABLED=0 go build -o oci-dyndns -ldflags '-w -extldflags "-static"' cmd/main.go

FROM alpine:3.23
ARG UID=1000
ARG GID=1000

RUN adduser -u ${UID} -g ${GID} app -D
COPY --from=build /workspace/oci-dyndns /usr/local/bin/oci-dyndns
USER ${UID}

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/oci-dyndns", "--config", "/config.json"]

LABEL org.opencontainers.image.title=OCI-DynDNS
LABEL org.opencontainers.image.description="Oracle Cloud DNS updater via HTTP API"
LABEL org.opencontainers.image.source=https://github.com/haimgel/oci-dyndns
LABEL org.opencontainers.image.licenses=MIT
