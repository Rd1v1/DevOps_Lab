#!/bin/sh
set -eu
cert_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/ssl"
mkdir -p "$cert_dir"
umask 077
openssl req -x509 -nodes -newkey rsa:2048 -days 365 \
    -keyout "$cert_dir/server.key" -out "$cert_dir/server.crt" \
    -subj "/CN=devops-stack.local" \
    -addext "subjectAltName=DNS:devops-stack.local,DNS:localhost,IP:127.0.0.1"
chmod 600 "$cert_dir/server.key"
chmod 644 "$cert_dir/server.crt"
