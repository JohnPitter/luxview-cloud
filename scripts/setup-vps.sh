#!/usr/bin/env bash
# =============================================================================
# LuxView Cloud — VPS Initial Setup Script
# =============================================================================
# Target: Ubuntu 22.04 LTS (fresh VPS)
# Run as root: curl -sSL <url> | bash
# =============================================================================

set -euo pipefail

LOG_PREFIX="[luxview-setup]"
log() { echo "$LOG_PREFIX $1"; }

# -- Sanity check -------------------------------------------------------------
if [[ $EUID -ne 0 ]]; then
    echo "ERROR: This script must be run as root."
    exit 1
fi

log "Starting VPS setup..."

# -- 1. System update ---------------------------------------------------------
log "Updating system packages..."
apt-get update -qq
apt-get upgrade -y -qq

# -- 2. Essential packages ----------------------------------------------------
log "Installing essential packages..."
apt-get install -y -qq \
    curl \
    wget \
    git \
    unzip \
    htop \
    ncdu \
    jq \
    ca-certificates \
    gnupg \
    lsb-release \
    fail2ban

# -- 3. Docker Engine ---------------------------------------------------------
log "Installing Docker Engine..."
if ! command -v docker &>/dev/null; then
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
        gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg

    echo \
        "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
        https://download.docker.com/linux/ubuntu \
        $(lsb_release -cs) stable" | \
        tee /etc/apt/sources.list.d/docker.list > /dev/null

    apt-get update -qq
    apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
    systemctl enable docker
    systemctl start docker
    log "Docker installed: $(docker --version)"
else
    log "Docker already installed: $(docker --version)"
fi

# -- 4. Firewall (UFW) --------------------------------------------------------
log "Configuring UFW firewall..."
apt-get install -y -qq ufw

ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp    comment "SSH"
ufw allow 80/tcp    comment "HTTP"
ufw allow 443/tcp   comment "HTTPS"
ufw --force enable
ufw status verbose
log "Firewall configured (SSH, HTTP, HTTPS only)."

# -- 5. Swap file (2 GB) ------------------------------------------------------
SWAP_SIZE="2G"
if ! swapon --show | grep -q "/swapfile"; then
    log "Creating ${SWAP_SIZE} swap file..."
    fallocate -l "$SWAP_SIZE" /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo "/swapfile none swap sw 0 0" >> /etc/fstab
    # Optimize swap behavior
    sysctl vm.swappiness=10
    echo "vm.swappiness=10" >> /etc/sysctl.conf
    sysctl vm.vfs_cache_pressure=50
    echo "vm.vfs_cache_pressure=50" >> /etc/sysctl.conf
    log "Swap enabled."
else
    log "Swap already active."
fi

# -- 6. Kernel tuning for Docker + networking ---------------------------------
log "Applying kernel tuning..."
cat >> /etc/sysctl.conf <<'SYSCTL'

# LuxView Cloud — kernel tuning
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535
net.ipv4.ip_local_port_range = 1024 65535
net.ipv4.tcp_tw_reuse = 1
fs.file-max = 2097152
fs.inotify.max_user_watches = 524288
SYSCTL
sysctl -p

# -- 7. Create luxview user ---------------------------------------------------
if ! id "luxview" &>/dev/null; then
    log "Creating 'luxview' user..."
    useradd -m -s /bin/bash -G docker luxview
    log "User 'luxview' created and added to docker group."
else
    log "User 'luxview' already exists."
fi

# -- 8. Project directory -----------------------------------------------------
PROJECT_DIR="/opt/luxview-cloud"
log "Creating project directory at ${PROJECT_DIR}..."
mkdir -p "$PROJECT_DIR"
chown luxview:luxview "$PROJECT_DIR"

# -- 9. Backup directory ------------------------------------------------------
mkdir -p /backups
chown luxview:luxview /backups
log "Backup directory created at /backups."

# -- 10. Fail2ban for SSH brute-force protection ------------------------------
log "Configuring fail2ban..."
cat > /etc/fail2ban/jail.local <<'JAIL'
[sshd]
enabled = true
port = ssh
filter = sshd
logpath = /var/log/auth.log
maxretry = 5
bantime = 3600
findtime = 600
JAIL
systemctl enable fail2ban
systemctl restart fail2ban

# -- 11. Docker log rotation ---------------------------------------------------
log "Configuring Docker log rotation..."
cat > /etc/docker/daemon.json <<'DOCKER'
{
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "10m",
        "max-file": "3"
    },
    "default-ulimits": {
        "nofile": {
            "Name": "nofile",
            "Hard": 65536,
            "Soft": 65536
        }
    }
}
DOCKER
systemctl restart docker

# -- Done ---------------------------------------------------------------------
log "==========================================="
log " VPS setup complete!"
log "==========================================="
log ""
log " Next steps:"
log "   1. Clone repo:  git clone <repo> ${PROJECT_DIR}"
log "   2. Copy env:    cp .env.example .env && vim .env"
log "   3. DNS:         Point luxview.cloud + *.luxview.cloud to this IP"
log "   4. Start:       cd ${PROJECT_DIR} && make prod"
log "   5. Migrate:     make migrate"
log ""
