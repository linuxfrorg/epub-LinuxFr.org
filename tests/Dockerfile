# syntax=docker/dockerfile:1

FROM ghcr.io/orange-opensource/hurl:6.1.1

LABEL "org.opencontainers.image.source"="https://github.com/linuxfrorg/img-LinuxFr.org"
LABEL "org.opencontainers.image.description"="Tests suite for the the fly epub3 producer from a content on LinuxFr.org and its comments"
LABEL "org.opencontainers.image.licenses"="AGPL-3.0-only"

ARG EPUBCHECK_VERSION=5.2.1
ARG DEBUG=1

WORKDIR /app

COPY tests_misc.hurl ./
COPY tests_epub.hurl ./
COPY epub-tests.sh ./

# hadolint ignore=SC2016
RUN set -eux; \
  apk add --no-cache \
    bash=5.2.37-r0 \
    curl=8.12.1-r1 \
    openjdk17=17.0.14_p7-r0 \
    openssl=3.3.3-r0 \
    shellcheck=0.10.0-r1 \
    unzip=6.0-r15 \
    bind-tools=9.18.35-r0 \
  && shellcheck /app/epub-tests.sh \
  && curl --fail --silent --show-error --location \
    -o "/tmp/epubcheck-$EPUBCHECK_VERSION.zip" \
    --url "https://github.com/w3c/epubcheck/releases/download/v${EPUBCHECK_VERSION}/epubcheck-$EPUBCHECK_VERSION.zip" \
  && unzip -q -o "/tmp/epubcheck-$EPUBCHECK_VERSION.zip" -d /app \
  && apk del --no-cache unzip \
  && rm -rf "/tmp/epubcheck-$EPUBCHECK_VERSION.zip" \
  && printf '#!/bin/bash\n java -jar /app/epubcheck-%s/epubcheck.jar "${@:1}"\n' "$EPUBCHECK_VERSION" > epubcheck \
  && chmod +x epubcheck \
  && mkdir -p cert-web epub

ENV PATH="$PATH:/app"

ENTRYPOINT ["/app/epub-tests.sh"]
