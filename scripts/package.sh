#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
test -f src/main.go
test -f src/main_test.go
test -f src/go.mod
for f in launch.sh config.json icon.png ota.json cacert.pem; do
  test -s "app/$f"
done
test -d app/assets

VERSION="$(sed -n 's/.*appVersion[[:space:]]*=[[:space:]]*"\(v[0-9][0-9.]*\)".*/\1/p' src/main.go | head -1)"
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+(\.[0-9]+)?$ ]]; then
  echo "Invalid appVersion in src/main.go: $VERSION" >&2
  exit 1
fi
RELEASE_TAG="$(printenv RELEASE_TAG || true)"
if [[ -n "$RELEASE_TAG" && "$RELEASE_TAG" != "$VERSION" ]]; then
  echo "Requested release $RELEASE_TAG does not equal source appVersion $VERSION" >&2
  exit 1
fi

mkdir -p dist
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
PKG="$TMP/Apps/BinanceGia.pak"
mkdir -p "$PKG"
cp -a app/. "$PKG/"
chmod +x "$PKG/launch.sh"

(
  cd src
  GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$PKG/binance-gia" .
)
chmod +x "$PKG/binance-gia"

python3 - "$PKG" <<'PY'
import json, pathlib, sys
folder = pathlib.Path(sys.argv[1])
cfg = json.loads((folder/'config.json').read_text())
assert cfg['launch'] == 'launch.sh'
for path in ('icon.png','launch.sh','binance-gia','ota.json','cacert.pem'):
    assert (folder/path).is_file(), path
for path in ('favorites.json','settings.json','market-cache.json','binance-gia.log'):
    assert not (folder/path).exists(), f'Unexpected private file {path}'
print('Package configuration verified')
PY

OUT="$ROOT/dist/BinanceGia_TrimUI_StockOS_$VERSION.zip"
(cd "$TMP" && zip -q -X -r "$OUT" Apps)
unzip -tq "$OUT"
sha256sum "$OUT"
echo "Created: $OUT"
