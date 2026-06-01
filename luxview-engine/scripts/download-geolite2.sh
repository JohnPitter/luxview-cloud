#!/bin/sh
# Download free GeoLite2-compatible City database from DB-IP.
# License: CC BY 4.0 — https://db-ip.com/db/download/ip-to-city-lite
#
# NON-FATAL by design: a failed download must never break the engine build. The
# DB is only used for analytics geolocation, which degrades gracefully when the
# file is absent. DB-IP publishes each month's file a few days into the month, so
# on (or near) the 1st we fall back to the previous month.

DEST_DIR="${1:-/usr/share/GeoIP}"
mkdir -p "$DEST_DIR"

Y=$(date +%Y)
M=$(date +%m | sed 's/^0*//')
[ -z "$M" ] && M=1

cur=$(printf '%04d-%02d' "$Y" "$M")
pm=$((M - 1)); py=$Y
if [ "$pm" -lt 1 ]; then pm=12; py=$((Y - 1)); fi
prev=$(printf '%04d-%02d' "$py" "$pm")

for MONTH in "$cur" "$prev"; do
  URL="https://download.db-ip.com/free/dbip-city-lite-${MONTH}.mmdb.gz"
  echo "Trying GeoIP database $URL ..."
  if wget -q -O /tmp/geoip.mmdb.gz "$URL"; then
    if gunzip -f /tmp/geoip.mmdb.gz && mv /tmp/geoip.mmdb "$DEST_DIR/GeoLite2-City.mmdb"; then
      echo "GeoIP database installed at $DEST_DIR/GeoLite2-City.mmdb"
      exit 0
    fi
  fi
  echo "  -> not available, trying next..."
done

echo "WARNING: could not download a GeoIP database; continuing without geolocation."
exit 0
