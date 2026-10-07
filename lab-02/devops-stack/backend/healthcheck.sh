#!/bin/sh
set -eu
exec wget -q -O /dev/null -T 2 http://127.0.0.1:8000/health
