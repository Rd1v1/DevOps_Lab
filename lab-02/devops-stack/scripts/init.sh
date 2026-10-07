#!/bin/sh
set -eu
cd "$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
if [ ! -f .env ]; then
    umask 077
    password="$(openssl rand -hex 32)"
    printf 'DB_PASSWORD=%s\nHTTPS_PORT=8443\nHTTP_PORT=8088\n' "$password" > .env
fi
if [ ! -f nginx/ssl/server.key ] || [ ! -f nginx/ssl/server.crt ]; then
    sh nginx/generate-cert.sh
fi
docker compose config --quiet
printf '%s\n' 'Ready: docker compose -p devops-lab02 up -d --build --wait'
