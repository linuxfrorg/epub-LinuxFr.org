#!/usr/bin/env sh

set -eu

printf "Check certificate for %s %s %s\n" "$WEB_NAME" "$WEB_IP4" "$WEB_IP6"

CERT_DIR="/app/private"
CA_DIR="/app/certs"
CA_KEY="$CERT_DIR/ca.key"
CA_CRT="$CERT_DIR/ca.crt"
WEB_CSR="$CERT_DIR/web.csr"
WEB_CRT="$CERT_DIR/web.crt"
WEB_KEY="$CERT_DIR/web.key"
WEB_DH="$CERT_DIR/dh2048.pem"
WEB_EXT="$CERT_DIR/web.ext"
EPUB_CA="$CA_DIR/ca-certificates.crt"

if [ ! -f "$WEB_CRT" ] ; then
  echo "First run, producing crypto stuff for https"
  mkdir -p "$CERT_DIR" "$CA_DIR"
  openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:secp384r1 -days 3650 \
    -nodes -keyout "$CA_KEY" -out "$CA_CRT" -subj "/CN=CA"
  openssl req -new -newkey ec -pkeyopt ec_paramgen_curve:secp384r1 \
    -nodes -keyout "$WEB_KEY" -out "$WEB_CSR" -subj "/CN=$WEB_NAME"

  cat > "$WEB_EXT" << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = $WEB_NAME
IP.1 = $WEB_IP4
IP.2 = $WEB_IP6
EOF

  openssl x509 -req -in "$WEB_CSR" -CA "$CA_CRT" -CAkey "$CA_KEY" -CAcreateserial -out "$WEB_CRT" -days 3650 -sha256 -extfile "$WEB_EXT"
  rm "${CA_KEY}" # not needed anymore
  chmod a+r "${WEB_KEY}" "${WEB_EXT}" # do not do that on production
  cp "$CA_CRT" "$EPUB_CA" # override all CAs
  openssl dhparam -out "$WEB_DH" "2048"
  echo "Crypto stuff produced."
fi
exit 0
