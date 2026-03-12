#!/bin/bash
# =============================================================================
# LuxView Cloud — Mail Server Initial Setup
# Run this ONCE after first deploy to configure SSL + DKIM
# Usage: bash scripts/setup-mailserver.sh
# =============================================================================

set -e

MAIL_CONTAINER="luxview-mailserver"
DOMAIN="luxview.cloud"

echo "=== LuxView Mail Server Setup ==="

# 1. Wait for mailserver to be ready
echo "[1/4] Waiting for mailserver container..."
until docker exec "$MAIL_CONTAINER" true 2>/dev/null; do
  sleep 2
done
echo "  -> Container ready"

# 2. Generate DKIM keys
echo "[2/4] Generating DKIM keys..."
docker exec "$MAIL_CONTAINER" setup config dkim keysize 2048 domain "$DOMAIN"
echo "  -> DKIM keys generated"

# 3. Display DKIM record for DNS
echo ""
echo "=== ADD THIS DKIM RECORD TO YOUR DNS ==="
echo ""
docker exec "$MAIL_CONTAINER" cat /tmp/docker-mailserver/opendkim/keys/"$DOMAIN"/mail.txt 2>/dev/null || \
  echo "(DKIM key will be available after container restart)"
echo ""

# 4. Display required DNS records
echo "=== REQUIRED DNS RECORDS ==="
echo ""
echo "Type   | Name                          | Value"
echo "-------|-------------------------------|------"
echo "A      | mail.luxview.cloud            | <YOUR_VPS_IP>"
echo "MX     | luxview.cloud                 | mail.luxview.cloud (priority 10)"
echo "TXT    | luxview.cloud                 | v=spf1 a mx ip4:<YOUR_VPS_IP> ~all"
echo "TXT    | _dmarc.luxview.cloud          | v=DMARC1; p=quarantine; rua=mailto:admin@luxview.cloud"
echo "TXT    | mail._domainkey.luxview.cloud  | (DKIM key above)"
echo ""
echo "=== IMPORTANT ==="
echo "1. Replace <YOUR_VPS_IP> with your actual VPS IP address"
echo "2. Add ALL DNS records before sending emails"
echo "3. Check porta 25 is open: telnet mail.luxview.cloud 25"
echo "4. Test with: https://mxtoolbox.com/SuperTool.aspx"
echo ""

# 5. Create postmaster account
echo "[3/4] Creating postmaster account..."
docker exec "$MAIL_CONTAINER" setup email add "admin@$DOMAIN" "$(openssl rand -base64 24)" 2>/dev/null || true
echo "  -> Postmaster account created"

echo "[4/4] Setup complete! Restart mailserver for DKIM to take effect:"
echo "  docker restart $MAIL_CONTAINER"
