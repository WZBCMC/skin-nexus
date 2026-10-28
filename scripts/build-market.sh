#!/usr/bin/env bash
# Build signed bundles + registry locally. The private key never enters the repo.
set -euo pipefail

: "${SKIN_PLUGIN_KEY:?set SKIN_PLUGIN_KEY to the offline PKCS#8 PEM path}"
: "${SKIN_PLUGIN_KEY_ID:?set SKIN_PLUGIN_KEY_ID}"
: "${MARKET_URL_BASE:?set MARKET_URL_BASE to https://<pages-host>/skin-nexus-plugins/v1/artifacts}"
: "${MARKET_DIST:?set MARKET_DIST to the dist directory}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT/server"
mkdir -p "$MARKET_DIST"
rm -f "$MARKET_DIST"/*.zip "$MARKET_DIST/registry.json"
for name in config-greeter nav-links player-notes; do
  go run ./cmd/skin-plugin pack \
    --dir "../examples/plugins/$name" \
    --out "$MARKET_DIST/$name-1.0.0.zip" \
    --key "$SKIN_PLUGIN_KEY" --key-id "$SKIN_PLUGIN_KEY_ID" --json
done
go run ./cmd/skin-plugin registry \
  --dist "$MARKET_DIST" --url-base "$MARKET_URL_BASE" \
  --key "$SKIN_PLUGIN_KEY" --key-id "$SKIN_PLUGIN_KEY_ID" \
  --out "$MARKET_DIST/registry.json" --json
if [[ -n "${SKIN_PLUGIN_TRUST_KEYS:-}" ]]; then
  go run ./cmd/skin-plugin verify-registry --registry "$MARKET_DIST/registry.json" --keys "$SKIN_PLUGIN_TRUST_KEYS" --json
fi
echo "Market artifacts ready in $MARKET_DIST"
