#!/bin/sh
# Read-only TG4040 LED diagnostics. Nothing is written to the sysfs driver.
echo "=== Device ==="
tr '\000' ' ' </proc/device-tree/model 2>/dev/null || true
echo
uname -a 2>/dev/null || true
echo "=== LED driver nodes ==="
if [ ! -d /sys/class/led_anim ]; then
  echo "Missing /sys/class/led_anim"
  exit 0
fi
ls -la /sys/class/led_anim 2>/dev/null
echo "=== LED help and effect names ==="
for node in help effect_names; do
  f="/sys/class/led_anim/$node"
  if [ -r "$f" ]; then
    echo "--- $node ---"
    head -c 10000 "$f" 2>/dev/null || true
    echo
  fi
done
echo "=== Battery nodes ==="
for f in /sys/class/power_supply/*/type /sys/class/power_supply/*/capacity; do
  [ -r "$f" ] || continue
  printf '%s: ' "$f"
  cat "$f" 2>/dev/null
done
