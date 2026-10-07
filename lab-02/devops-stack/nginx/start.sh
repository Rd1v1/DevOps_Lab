#!/bin/sh
set -eu
export HTTPS_PORT="${HTTPS_PORT:-8443}"
envsubst '${HTTPS_PORT}' < /etc/nginx/conf.d/default.conf > /tmp/default.conf
exec nginx -g 'daemon off;'
