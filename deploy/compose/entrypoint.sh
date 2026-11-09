#!/bin/sh
set -eu

if [ -n "${BOOKMARKER_CONFIG:-}" ]; then
    exec "$@"
fi

if [ -z "${BOOKMARKER_ADMIN_USERNAME:-}" ] || [ -z "${BOOKMARKER_ADMIN_PASSWORD_HASH:-}" ] || [ -z "${BOOKMARKER_DB_PASSWORD:-}" ]; then
    printf '%s\n' 'missing required environment setting' >&2
    exit 1
fi

runtime_dir=/run/bookmarker
config_path="$runtime_dir/config.yaml"
mkdir -p "$runtime_dir"
umask 077
chmod 700 "$runtime_dir"
bookmarker-config-init

export BOOKMARKER_CONFIG="$config_path"
exec "$@"