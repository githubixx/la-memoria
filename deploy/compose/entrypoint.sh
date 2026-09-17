#!/bin/sh
set -eu

for name in BOOKMARKER_ADMIN_USERNAME BOOKMARKER_ADMIN_PASSWORD_HASH BOOKMARKER_DB_PASSWORD; do
    eval "value=\${$name-}"
    if [ -z "$value" ]; then
        printf '%s\n' "missing required environment setting: $name" >&2
        exit 1
    fi
done

if ! printf '%s' "$BOOKMARKER_ADMIN_USERNAME" | grep -Eq '^[A-Za-z0-9._-]+$'; then
    printf '%s\n' 'BOOKMARKER_ADMIN_USERNAME must contain only letters, digits, dots, underscores, or hyphens' >&2
    exit 1
fi

if ! printf '%s' "$BOOKMARKER_ADMIN_PASSWORD_HASH" | grep -Eq '^\$argon2id\$v=19\$m=65536,t=3,p=1\$[A-Za-z0-9+/]+\$[A-Za-z0-9+/]+$'; then
    printf '%s\n' 'BOOKMARKER_ADMIN_PASSWORD_HASH must be a supported Argon2id PHC verifier' >&2
    exit 1
fi

runtime_dir=/run/bookmarker
config_path="$runtime_dir/config.yaml"
mkdir -p "$runtime_dir"
umask 077
sed \
    -e "s|__BOOKMARKER_ADMIN_USERNAME__|$BOOKMARKER_ADMIN_USERNAME|g" \
    -e "s|__BOOKMARKER_ADMIN_PASSWORD_HASH__|$BOOKMARKER_ADMIN_PASSWORD_HASH|g" \
    /app/config.yaml.tmpl > "$config_path"
chmod 600 "$config_path"

export BOOKMARKER_CONFIG="$config_path"
exec "$@"