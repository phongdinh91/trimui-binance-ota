#!/bin/sh
APPDIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
cd "$APPDIR" || exit 1

chmod +x ./binance-gia 2>/dev/null
export SSL_CERT_FILE="$APPDIR/cacert.pem"
export GODEBUG="netdns=go"

MAINUI_PIDS="$(pidof MainUI 2>/dev/null)"
restore_mainui() {
  if [ -n "$MAINUI_PIDS" ]; then
    for p in $MAINUI_PIDS; do kill -CONT "$p" 2>/dev/null; done
  fi
}
trap restore_mainui EXIT INT TERM

if [ -n "$MAINUI_PIDS" ]; then
  for p in $MAINUI_PIDS; do kill -STOP "$p" 2>/dev/null; done
fi

./binance-gia >> "$APPDIR/binance-gia.log" 2>&1
exit $?
