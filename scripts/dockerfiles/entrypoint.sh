#!/bin/sh
set -e

PUID="${PUID:-0}"
PGID="${PGID:-0}"

cd /app

if [ "$PUID" != "0" ] || [ "$PGID" != "0" ]; then
    # Custom UID/GID requested: chown and switch user
    chown -R "$PUID:$PGID" /app

    if command -v su-exec >/dev/null 2>&1; then
        exec su-exec "$PUID:$PGID" ./octopus start
    elif command -v gosu >/dev/null 2>&1; then
        exec gosu "$PUID:$PGID" ./octopus start
    else
        echo "Warning: neither su-exec nor gosu is available; running as current user." >&2
        exec ./octopus start
    fi
else
    # Default: run as current user (Dockerfile USER already set)
    exec ./octopus start
fi
