EPUB3 for LinuxFr.org contents
==============================

This daemon creates on the fly epub3 from a content on LinuxFr.org and its
comments.


How to use it? (outside container)
--------------

[Install Go](http://golang.org/doc/install) and don't forget to set `$GOPATH`

    # aptitude install libonig-dev libxml2-dev pkg-config
    $ go get -u github.com/linuxfrorg/epub-LinuxFr.org
    $ epub-LinuxFr.org [-addr addr] [-l logs] [-H host]

And, to display the help:

    $ epub-LinuxFr.org -h

How to use it? (with a container)
---------------------------------

Build and run container image (static binary by default):

    $ docker build --tag linuxfr.org-epub --file Containerfile .
    $ docker run --publish 9000:9000 linuxfr.org-epub

or

    $ buildah build --tag linuxfr.org-epub --file Containerfile .
    $ podman run --publish 9000:9000 linuxfr.org-epub

With dynamic binary:

    $ docker build --tag linuxfr.org-epub-dyn --file Containerfile.dynamic .

or

    $ buildah build --tag linuxfr.org-epub-dyn --file Containerfile.dynamic .

How it works?
-------------

Accepted requests are:
- `GET /status` (expected answer is HTTP 200 with "OK" body)
- get the corresponding content + comments on host site, converted into EPUB
  - `GET /news/<slug>.epub` (news)
  - `GET /users/<user>/journaux/<slug>.epub` (diary)
  - `GET /forums/<forum>/posts/<slug>.epub` (post / forum entry)
  - `GET /sondages/<slug>.epub` (poll)
  - `GET /suivi/<slug>.epub` (tracker entry)
  - `GET /wiki/<slug>.epub` (wiki page)
- otherwise HTTP 404

Caveats
-------

- require https for host
- not statically built (so libxml2, libonig2 and probably ca-certificates needed for deployment)
- answers HTTP 404 when something is wrong (unable to fetch, bad HTTP verb, bad content type, etc.)
- base64 inline images are not supported (log "Error: Get data:image/svg+xml;base64,...%0A: unsupported protocol scheme "data")

Testsuite
---------
Testsuite requires `docker-compose` or `podman-compose`.

```bash
cd tests/
docker-compose --file compose.yaml up --build
```

or

```bash
podman-compose --file compose.yaml up --build
```

Extra checks
------------

nb: you may use `podman` instead of `docker`

Linter for Containerfile:

```bash
for image in Containerfile Containerfile.dynamic tests/Containerfile tests/cert-web/Containerfile
do
  # Test with pinned hadolint/hadolint:v2.14.0-debian
  docker run --rm --interactive docker.io/hadolint/hadolint@sha256:158cd0184dcaa18bd8ec20b61f4c1cabdf8b32a592d062f57bdcb8e4c1d312e2 < "$image"
  # Test with replicated/dockerfilelint but last push more than 5 years ago...
  # docker run --rm --volume $(pwd)/$image:/app/Containerfile --workdir /app docker.io/replicated/dockerfilelint@sha256:15ce784e5847966b6d9a88cba348a9429f8b5212f6017180f10ce36b472dfe52 Containerfile
done
```

Linter for Go:

```bash
# (already embedded in Containerfile due to prerequisites)
# docker run --rm --tty --volume $(pwd):/app --workdir /app golangci/golangci-lint:vx.y.z golangci-lint run -v
```

Vulnerability/secret scanners:

```bash
docker run --rm --volume $(pwd):/app --workdir /app docker.io/aquasec/trivy:0.71.2 repo --skip-files cert-web/private/web.key .
docker run --rm --volume $(pwd):/app --workdir /app docker.io/chainguard/grype:latest --name linuxfr.org-epub dir:/app
```

See also
--------

* [Git repository](https://github.com/linuxfrorg/epub-LinuxFr.org)


Copyright
---------

The code is licensed as GNU AGPLv3. See the LICENSE file for the full license.

♡2013 by Bruno Michel. Copying is an act of love. Please copy and share.

2024-2026 by Benoît Sibaud and Adrien Dorsaz.
