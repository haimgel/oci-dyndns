ARG GOLANG_VERSION=1.18.6
ARG ALPINE_VERSION=3.16

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

COPY --from=build /workspace/oci-dyndns /usr/local/bin/oci-dyndns

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/oci-dyndns", "--config", "/config.json"]
