# syntax=docker/dockerfile:1

FROM docker.io/alpine:3.20.3

LABEL "org.opencontainers.image.source"="https://github.com/linuxfrorg/img-LinuxFr.org"
LABEL "org.opencontainers.image.description"="Certificates for tests"
LABEL "org.opencontainers.image.licenses"="AGPL-3.0-only"

COPY cert-web.sh /usr/local/bin

RUN apk add --no-cache \
    openssl=3.3.2-r1 \
    shellcheck=0.10.0-r1 \
  && shellcheck /usr/local/bin/cert-web.sh

WORKDIR /app

ARG WEB_NAME
ARG WEB_IP1
ARG WEB_IP2

ENTRYPOINT ["/usr/local/bin/cert-web.sh"]
