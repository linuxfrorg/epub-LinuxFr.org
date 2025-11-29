# syntax=docker/dockerfile:1

FROM ghcr.io/orange-opensource/hurl:7.1.0

LABEL "org.opencontainers.image.source"="https://github.com/linuxfrorg/img-LinuxFr.org"
LABEL "org.opencontainers.image.description"="Tests suite for the the fly epub3 producer from a content on LinuxFr.org and its comments"
LABEL "org.opencontainers.image.licenses"="AGPL-3.0-only"

ARG EPUBCHECK_VERSION=5.3.0
ARG DEBUG=1

WORKDIR /app

COPY tests_misc.hurl ./
COPY tests_epub.hurl ./
COPY epub-tests.sh ./

RUN set -eux; \
  apk add --no-cache \
    bash=5.2.37-r0 \
    curl=8.14.1-r2 \
    openjdk17=17.0.17_p10-r0 \
    openssl=3.5.4-r0 \
    shellcheck=0.10.0-r2 \
    unzip=6.0-r15 \
    bind-tools=9.20.16-r0 \
  && shellcheck /app/epub-tests.sh
# hadolint ignore=SC2016
RUN curl --fail --verbose --show-error --location \
    --output "/tmp/epubcheck-$EPUBCHECK_VERSION.zip" \
    --url "https://github.com/w3c/epubcheck/releases/download/v${EPUBCHECK_VERSION}/epubcheck-$EPUBCHECK_VERSION.zip" \
  && unzip -q -o "/tmp/epubcheck-$EPUBCHECK_VERSION.zip" -d /app \
  && apk del --no-cache unzip \
  && rm -rf "/tmp/epubcheck-$EPUBCHECK_VERSION.zip" \
  && printf '#!/bin/bash\n java -jar /app/epubcheck-%s/epubcheck.jar "${@:1}"\n' "$EPUBCHECK_VERSION" > epubcheck \
  && chmod +x epubcheck \
  && mkdir -p cert-web epub

ENV PATH="$PATH:/app"

ENTRYPOINT ["/app/epub-tests.sh"]
