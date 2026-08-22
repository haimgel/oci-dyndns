# Reference: https://hub.docker.com/_/golang/tags?name=1.26-alpine
#
# Name base images inline: Dependabot cannot resolve a tag behind an ARG (dependabot-core#2057).
# Keep the builder on the bare `-alpine` tag: the `-alpine<version>` variants stop being rebuilt once the Go
# team's default Alpine moves on, which freezes the toolchain. Only the runtime base below is pinned.
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

FROM alpine:3.24
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
