#!/bin/bash
#===============================================================================
# Vultr Server Bootstrap Script
# Run as root on a fresh Ubuntu 24.04 LTS instance
#
# Usage: curl -sSL https://raw.githubusercontent.com/.../bootstrap.sh | sudo bash
#        OR: sudo bash bootstrap.sh
#===============================================================================

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

#-------------------------------------------------------------------------------
# Configuration
#-------------------------------------------------------------------------------
DOCKER_USER="docker"
DOCKER_UID=1000
APP_DIR="/opt/conversational-platform"
DATA_DIR="/var/lib/platform-data"

#-------------------------------------------------------------------------------
# Pre-flight checks
#-------------------------------------------------------------------------------
if [[ $EUID -ne 0 ]]; then
   error "This script must be run as root"
fi

log "Starting Vultr server bootstrap..."
log "Creating secure 'docker' user and installing dependencies..."

#-------------------------------------------------------------------------------
# System Updates
#-------------------------------------------------------------------------------
log "Updating system packages..."
apt-get update && apt-get upgrade -y

#-------------------------------------------------------------------------------
# Install Essential Packages
#-------------------------------------------------------------------------------
log "Installing essential packages..."
apt-get install -y \
    curl \
    wget \
    git \
    jq \
    htop \
    vim \
    ufw \
    fail2ban \
    unattended-upgrades \
    apt-transport-https \
    ca-certificates \
    gnupg \
    lsb-release \
    software-properties-common

#-------------------------------------------------------------------------------
# Create Docker User (Security Best Practice)
#-------------------------------------------------------------------------------
log "Creating 'docker' user..."
if id "$DOCKER_USER" &>/dev/null; then
    warn "User '$DOCKER_USER' already exists, skipping creation"
else
    useradd -m -s /bin/bash -u $DOCKER_UID $DOCKER_USER
    log "Created user '$DOCKER_USER' with UID $DOCKER_UID"
fi

# Create .ssh directory for docker user
mkdir -p /home/$DOCKER_USER/.ssh
chmod 700 /home/$DOCKER_USER/.ssh

# Copy root's authorized_keys if exists (for SSH access)
if [[ -f /root/.ssh/authorized_keys ]]; then
    cp /root/.ssh/authorized_keys /home/$DOCKER_USER/.ssh/
    chmod 600 /home/$DOCKER_USER/.ssh/authorized_keys
    chown -R $DOCKER_USER:$DOCKER_USER /home/$DOCKER_USER/.ssh
    log "Copied SSH keys to docker user"
fi

#-------------------------------------------------------------------------------
# Install Docker
#-------------------------------------------------------------------------------
log "Installing Docker..."
if command -v docker &>/dev/null; then
    warn "Docker already installed, skipping"
else
    # Add Docker's official GPG key
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc

    # Add Docker repository
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
      $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
      tee /etc/apt/sources.list.d/docker.list > /dev/null

    apt-get update
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

    # Add docker user to docker group
    usermod -aG docker $DOCKER_USER
    log "Docker installed successfully"
fi

#-------------------------------------------------------------------------------
# Install Caddy (Reverse Proxy with Auto-HTTPS)
#-------------------------------------------------------------------------------
log "Installing Caddy..."
if command -v caddy &>/dev/null; then
    warn "Caddy already installed, skipping"
else
    apt-get install -y debian-keyring debian-archive-keyring apt-transport-https
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
    apt-get update
    apt-get install -y caddy
    log "Caddy installed successfully"
fi

#-------------------------------------------------------------------------------
# Create Application Directories
#-------------------------------------------------------------------------------
log "Creating application directories..."
mkdir -p $APP_DIR/{config,certs,logs}
mkdir -p $DATA_DIR/{nats,postgres}
chown -R $DOCKER_USER:$DOCKER_USER $APP_DIR
chown -R $DOCKER_USER:$DOCKER_USER $DATA_DIR

#-------------------------------------------------------------------------------
# Configure Firewall (UFW)
#-------------------------------------------------------------------------------
log "Configuring firewall..."
ufw --force reset
ufw default deny incoming
ufw default allow outgoing

# SSH (change to your IP for production!)
ufw allow 22/tcp comment 'SSH'

# HTTP/HTTPS (Caddy)
ufw allow 80/tcp comment 'HTTP'
ufw allow 443/tcp comment 'HTTPS'

# NATS (restrict to specific IPs in production!)
# ufw allow from YOUR_API_SERVER_IP to any port 4222 proto tcp comment 'NATS'
ufw allow 4222/tcp comment 'NATS (restrict in production!)'

# Enable firewall
ufw --force enable
log "Firewall configured"

#-------------------------------------------------------------------------------
# Configure Fail2Ban
#-------------------------------------------------------------------------------
log "Configuring Fail2Ban..."
cat > /etc/fail2ban/jail.local << 'EOF'
[DEFAULT]
bantime = 1h
findtime = 10m
maxretry = 5

[sshd]
enabled = true
port = ssh
filter = sshd
logpath = /var/log/auth.log
maxretry = 3
bantime = 24h
EOF

systemctl enable fail2ban
systemctl restart fail2ban
log "Fail2Ban configured"

#-------------------------------------------------------------------------------
# Configure Automatic Security Updates
#-------------------------------------------------------------------------------
log "Configuring automatic security updates..."
cat > /etc/apt/apt.conf.d/50unattended-upgrades << 'EOF'
Unattended-Upgrade::Allowed-Origins {
    "${distro_id}:${distro_codename}";
    "${distro_id}:${distro_codename}-security";
    "${distro_id}ESMApps:${distro_codename}-apps-security";
    "${distro_id}ESM:${distro_codename}-infra-security";
};
Unattended-Upgrade::Package-Blacklist {
};
Unattended-Upgrade::AutoFixInterruptedDpkg "true";
Unattended-Upgrade::MinimalSteps "true";
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::Automatic-Reboot "false";
EOF

cat > /etc/apt/apt.conf.d/20auto-upgrades << 'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";
EOF

log "Automatic updates configured"

#-------------------------------------------------------------------------------
# System Hardening
#-------------------------------------------------------------------------------
log "Applying system hardening..."

# Disable root login via SSH
sed -i 's/^PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
sed -i 's/^#PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config

# Disable password authentication (key-only)
sed -i 's/^PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
sed -i 's/^#PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config

# Restart SSH
systemctl restart sshd

# Kernel hardening
cat >> /etc/sysctl.conf << 'EOF'

# Security hardening
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.default.rp_filter = 1
net.ipv4.icmp_echo_ignore_broadcasts = 1
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.default.send_redirects = 0
net.ipv4.conf.all.accept_source_route = 0
net.ipv4.conf.default.accept_source_route = 0
net.ipv4.tcp_syncookies = 1
EOF

sysctl -p

log "System hardening applied"

#-------------------------------------------------------------------------------
# Create Environment File Template
#-------------------------------------------------------------------------------
log "Creating environment file template..."
cat > $APP_DIR/config/.env.template << 'EOF'
# Server Configuration
SERVER_PORT=8080
LOG_LEVEL=info

# NATS Configuration
NATS_URL=nats://nats:4222
NATS_TOKEN=CHANGE_ME_GENERATE_SECURE_TOKEN

# JWT Configuration
JWT_SECRET=CHANGE_ME_GENERATE_32_CHAR_SECRET

# LLM Configuration (set at least one)
ANTHROPIC_API_KEY=
OPENAI_API_KEY=

# Rate Limiting
RATE_LIMIT_REQUESTS=60
RATE_LIMIT_WINDOW=1m

# Tracing (optional)
TRACING_ENABLED=false
TRACING_ENDPOINT=

# Domain Configuration
DOMAIN=api.yourdomain.com
EOF

chown $DOCKER_USER:$DOCKER_USER $APP_DIR/config/.env.template
log "Environment template created at $APP_DIR/config/.env.template"

#-------------------------------------------------------------------------------
# Create Docker Compose for Production
#-------------------------------------------------------------------------------
log "Creating production docker-compose..."
cat > $APP_DIR/docker-compose.prod.yml << 'EOF'
version: '3.8'

services:
  nats:
    image: nats:2.10-alpine
    container_name: nats
    restart: unless-stopped
    command: ["-js", "-sd", "/data", "-m", "8222"]
    ports:
      - "4222:4222"
      - "127.0.0.1:8222:8222"  # Monitoring on localhost only
    volumes:
      - nats_data:/data
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8222/healthz"]
      interval: 10s
      timeout: 5s
      retries: 3
    deploy:
      resources:
        limits:
          memory: 512M

  api:
    image: ghcr.io/YOUR_ORG/conversational-platform:latest
    container_name: api
    restart: unless-stopped
    ports:
      - "127.0.0.1:8080:8080"  # Only accessible via Caddy
    env_file:
      - ./config/.env
    depends_on:
      nats:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    deploy:
      resources:
        limits:
          memory: 512M

volumes:
  nats_data:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /var/lib/platform-data/nats
EOF

chown $DOCKER_USER:$DOCKER_USER $APP_DIR/docker-compose.prod.yml
log "Production docker-compose created"

#-------------------------------------------------------------------------------
# Create Caddy Configuration
#-------------------------------------------------------------------------------
log "Creating Caddy configuration..."
cat > /etc/caddy/Caddyfile << 'EOF'
# Global options
{
    email admin@yourdomain.com
    # Uncomment for staging certificates during testing
    # acme_ca https://acme-staging-v02.api.letsencrypt.org/directory
}

# API Server
api.yourdomain.com {
    # Reverse proxy to Go API
    reverse_proxy localhost:8080 {
        # SSE-friendly settings
        flush_interval -1
        transport http {
            read_buffer 8KB
            write_buffer 8KB
        }
    }

    # Security headers
    header {
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        X-XSS-Protection "1; mode=block"
        Referrer-Policy strict-origin-when-cross-origin
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        -Server
    }

    # Logging
    log {
        output file /var/log/caddy/api-access.log
        format json
    }
}

# Optional: NATS monitoring (internal access only)
# nats.yourdomain.com {
#     @allowed remote_ip 10.0.0.0/8 172.16.0.0/12 192.168.0.0/16
#     reverse_proxy @allowed localhost:8222
# }
EOF

mkdir -p /var/log/caddy
chown caddy:caddy /var/log/caddy

log "Caddy configuration created"

#-------------------------------------------------------------------------------
# Create Helper Scripts
#-------------------------------------------------------------------------------
log "Creating helper scripts..."

# Deploy script
cat > $APP_DIR/deploy.sh << 'DEPLOY_EOF'
#!/bin/bash
set -euo pipefail

cd /opt/conversational-platform

echo "Pulling latest images..."
docker compose -f docker-compose.prod.yml pull

echo "Stopping services..."
docker compose -f docker-compose.prod.yml down

echo "Starting services..."
docker compose -f docker-compose.prod.yml up -d

echo "Waiting for health checks..."
sleep 10

echo "Checking service status..."
docker compose -f docker-compose.prod.yml ps

echo "Deployment complete!"
DEPLOY_EOF

chmod +x $APP_DIR/deploy.sh
chown $DOCKER_USER:$DOCKER_USER $APP_DIR/deploy.sh

# Backup script
cat > $APP_DIR/backup.sh << 'BACKUP_EOF'
#!/bin/bash
set -euo pipefail

BACKUP_DIR="/var/backups/platform"
DATE=$(date +%Y%m%d_%H%M%S)
RETENTION_DAYS=7

mkdir -p $BACKUP_DIR

echo "Creating NATS backup..."
docker exec nats nats-server --signal stop
tar -czf "$BACKUP_DIR/nats_$DATE.tar.gz" -C /var/lib/platform-data nats
docker start nats

echo "Cleaning old backups..."
find $BACKUP_DIR -name "*.tar.gz" -mtime +$RETENTION_DAYS -delete

echo "Backup complete: nats_$DATE.tar.gz"
BACKUP_EOF

chmod +x $APP_DIR/backup.sh
chown $DOCKER_USER:$DOCKER_USER $APP_DIR/backup.sh

# Logs script
cat > $APP_DIR/logs.sh << 'LOGS_EOF'
#!/bin/bash
SERVICE=${1:-api}
docker compose -f /opt/conversational-platform/docker-compose.prod.yml logs -f $SERVICE
LOGS_EOF

chmod +x $APP_DIR/logs.sh
chown $DOCKER_USER:$DOCKER_USER $APP_DIR/logs.sh

log "Helper scripts created"

#-------------------------------------------------------------------------------
# Print Summary
#-------------------------------------------------------------------------------
echo ""
echo "=============================================================================="
echo -e "${GREEN}Bootstrap Complete!${NC}"
echo "=============================================================================="
echo ""
echo "Next steps:"
echo ""
echo "1. Switch to docker user:"
echo "   su - docker"
echo ""
echo "2. Copy your environment file:"
echo "   cp $APP_DIR/config/.env.template $APP_DIR/config/.env"
echo "   nano $APP_DIR/config/.env  # Edit with your values"
echo ""
echo "3. Update Caddyfile with your domain:"
echo "   sudo nano /etc/caddy/Caddyfile"
echo "   sudo systemctl reload caddy"
echo ""
echo "4. Deploy the application:"
echo "   cd $APP_DIR && ./deploy.sh"
echo ""
echo "5. Restrict firewall (IMPORTANT for production!):"
echo "   sudo ufw delete allow 4222/tcp"
echo "   sudo ufw allow from YOUR_API_SERVER_IP to any port 4222"
echo ""
echo "Security notes:"
echo "  - Root SSH login is DISABLED"
echo "  - Password auth is DISABLED (key-only)"
echo "  - Fail2Ban is protecting SSH"
echo "  - Auto security updates enabled"
echo ""
echo "Useful commands:"
echo "  ./deploy.sh     - Deploy/update application"
echo "  ./backup.sh     - Backup NATS data"
echo "  ./logs.sh api   - View API logs"
echo "  ./logs.sh nats  - View NATS logs"
echo ""
echo "=============================================================================="
