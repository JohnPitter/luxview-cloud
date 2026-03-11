#!/bin/sh
# Download free GeoLite2-compatible City database from DB-IP
# License: CC BY 4.0 — https://db-ip.com/db/download/ip-to-city-lite
set -e

DEST_DIR="${1:-/usr/share/GeoIP}"
mkdir -p "$DEST_DIR"

MONTH=$(date +%Y-%m)
URL="https://download.db-ip.com/free/dbip-city-lite-${MONTH}.mmdb.gz"

echo "Downloading GeoIP database from $URL..."
wget -q -O /tmp/geoip.mmdb.gz "$URL"
gunzip -f /tmp/geoip.mmdb.gz
mv /tmp/geoip.mmdb "$DEST_DIR/GeoLite2-City.mmdb"
echo "GeoIP database installed at $DEST_DIR/GeoLite2-City.mmdb"
