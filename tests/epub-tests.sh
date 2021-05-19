#!/usr/bin/env bash
#
set -eu -o pipefail

SCRIPT_DIR="$(dirname "$0")"
EPUB_DIR="${SCRIPT_DIR}/epub"

# Hosts from Docker compose file
EPUB="linuxfr.org-epub"

# shellcheck disable=SC2034
TARGET4="$(dig "${EPUB}" A +short)"        # epub IPv4
# shellcheck disable=SC2034
TARGET6="[$(dig "${EPUB}" AAAA +short)]" # epub IPv6

echo "Removing previously produced epub files if any"
rm -f -- "${EPUB_DIR}"/*.epub

for ip in 4 6
do
  for http2 in false true
  do
    target="TARGET$ip"
    hurl -$ip ${DEBUG:+-v} \
      --variable "TARGET=${!target}" \
      --variable "HTTP2=${http2}" \
      --test tests_misc.hurl tests_epub.hurl
  done
done

check_status=0
for epub in epub/*.epub
do
  printf "Check %s\n" "$epub"
  epubcheck -q "$epub" || check_status=1
done
for epub in "${EPUB_DIR}"/*.epub.broken
do
  printf "Check %s (failure expected)\n" "$epub"
  epubcheck -q "$epub" && check_status=1 || printf "Failed as expected\n"
done
if [ $check_status -ne 0 ] ; then
  printf "At least one invalid epub file\n"
  exit 3
fi

printf "All tests look good!\n"
