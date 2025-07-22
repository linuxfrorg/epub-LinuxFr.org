# syntax=docker/dockerfile:1

# Build
FROM docker.io/golang:1.24.5-bookworm AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    libonig-dev=6.9.8-1 \
    libxml2-dev=2.9.14+dfsg-1.3~deb12u2 \
    pkgconf=1.8.1-1 \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*

RUN go build -trimpath -o epub-LinuxFr.org

RUN go install golang.org/x/vuln/cmd/govulncheck@latest \
  && govulncheck -show verbose ./... \
  && govulncheck --mode=binary -show verbose epub-LinuxFr.org

# Lint
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN curl --fail --silent --show-error --location "https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh"|sh -s -- -b "$(go env GOPATH)"/bin v2.3.0 \
  && golangci-lint run -v

# Deploy
FROM docker.io/debian:bookworm
ARG UID=1000
ARG GID=1000
RUN addgroup --gid "${GID}" app \
  && adduser --disabled-password --comment '' --home /app --shell /bin/sh --uid "${UID}" --ingroup app app

LABEL "org.opencontainers.image.source"="https://github.com/linuxfrorg/epub-LinuxFr.org"
LABEL "org.opencontainers.image.description"="Produce on the fly epub3 from a content on LinuxFr.org and its comments"
LABEL "org.opencontainers.image.licenses"="AGPL-3.0-only"

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
  && apt-get install --assume-yes --no-install-recommends \
    libonig5=6.9.8-1 \
    libxml2=2.9.14+dfsg-1.3~deb12u1 \
    ca-certificates=20230311 \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*

USER app

WORKDIR /app

COPY --from=build --chown=app:app /app/epub-LinuxFr.org .

EXPOSE 9000

ENTRYPOINT ["/app/epub-LinuxFr.org"]
CMD ["--help"]
