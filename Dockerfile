# syntax=docker/dockerfile:1

FROM golangci/golangci-lint:v2.7.0-alpine AS lint

# prepare workaround for libonig.a not available in libonig-dev Debian package?!
FROM debian:trixie AS libonig-static

ARG UID=1000
ARG GID=1000

# hadolint ignore=DL3003
RUN sed -i 's/Types: deb/Types: deb deb-src/' /etc/apt/sources.list.d/debian.sources \
  && apt-get update \
  && apt-get build-dep --assume-yes --no-install-recommends libonig \
  && apt-get source libonig=6.9.9-1 \
  && cd libonig-* \
  && dpkg-buildpackage -us -uc -nc \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*

# Build
FROM docker.io/golang:1.25.5-trixie AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    libonig-dev=6.9.9-1+b1 \
    libxml2-dev=2.12.7+dfsg+really2.9.14-2.1+deb13u2 \
    liblzma-dev=5.8.1-1 \
    libzstd-dev:amd64=1.5.7+dfsg-1 \
    zlib1g-dev:amd64=1:1.3.dfsg+really1.3.1-1+b1 \
    libicu-dev=76.1-4 \
    pkgconf=1.8.1-4 \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*

# workaround for libonig.a not available in libonig-dev Debian package?!
COPY --from=libonig-static /libonig-6.9.9/src/.libs/libonig.a /usr/lib/x86_64-linux-gnu/

# we could dynamically built with dependances on libxml2, libonig5 and libc
# RUN go build -trimpath -o epub-LinuxFr.org
# but instead try a static build
# even if that case, use of dlopen, getaddrinfo & gethostbyname in dependencies
# 'requires at runtime the shared libraries from the glibc version used for linking'
# according to compiler/linker but we won't listen to anyway because because
# and deploy on alpine
RUN go vet \
  && GOOS=linux GOARCH=amd64 go build \
    -ldflags='-extldflags "-static -lz -licuuc -licutu -licuio -llzma -licudata -lstdc++ -lm" -w -L /usr/lib/x86_64-linux-gnu -L /usr/lib/gcc/x86_64-linux-gnu"' \
    -trimpath -o epub-LinuxFr.org \
  && ldd epub-LinuxFr.org || echo "OK not dynamic"

RUN go install golang.org/x/vuln/cmd/govulncheck@latest \
  && govulncheck -show verbose ./... \
  && govulncheck --mode=binary -show verbose epub-LinuxFr.org

# Lint
COPY --from=lint /usr/bin/golangci-lint "/go/bin/golangci-lint"
RUN golangci-lint run -v

# Deploy
FROM docker.io/alpine:3.22.2
ARG UID=1000
ARG GID=1000
RUN addgroup -g "${GID}" app \
  && adduser -D -g '' -h /app -s /bin/sh -u "${UID}" -G app app \
  && apk add --no-cache ca-certificates=20250911-r0
USER app


LABEL "org.opencontainers.image.source"="https://github.com/linuxfrorg/epub-LinuxFr.org"
LABEL "org.opencontainers.image.description"="Produce on the fly epub3 from a content on LinuxFr.org and its comments"
LABEL "org.opencontainers.image.licenses"="AGPL-3.0-only"

WORKDIR /app

COPY --from=build --chown=app:app /app/epub-LinuxFr.org .

EXPOSE 9000

ENTRYPOINT ["/app/epub-LinuxFr.org"]
CMD ["--help"]
