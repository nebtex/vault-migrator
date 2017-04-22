#!/usr/local/bin/dumb-init /bin/ash

set -e

chown vault-migrator:vault-migrator ${VAULT_MIGRATOR_CONFIG_FILE}

su-exec vault-migrator /bin/vault-migrator

