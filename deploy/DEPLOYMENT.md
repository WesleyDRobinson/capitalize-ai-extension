# Deployment Guide

Production deployment guide for the Conversational AI Platform on Vultr.

## Architecture Overview

```
                                   ┌─────────────────────────────────────────┐
                                   │           Vultr VPS                     │
                                   │                                         │
┌──────────┐    HTTPS    ┌─────────┴─────────┐                               │
│  Client  │────────────▶│  Caddy (443/80)   │                               │
│ (Vercel) │◀────────────│  Auto-TLS + Proxy │                               │
└──────────┘             └─────────┬─────────┘                               │
                                   │ :8080                                   │
                                   ▼                                         │
                         ┌───────────────────┐      ┌───────────────────┐    │
                         │    Go API         │◀────▶│  NATS JetStream   │    │
                         │  (Docker)         │      │  (Docker)         │    │
                         └───────────────────┘      └───────────────────┘    │
                                   │                         │               │
                                   │     Anthropic API       │               │
                                   └─────────────────────────┼───────────────┘
                                                             │
                                                     ┌───────┴───────┐
                                                     │ /var/lib/     │
                                                     │ platform-data │
                                                     └───────────────┘
```

## Why Caddy Instead of Envoy Gateway?

**TL;DR: Envoy is overkill for Series A scale. Use Caddy.**

### Envoy Gateway Would Add:
- Complex YAML configuration (100s of lines)
- Separate control plane management
- Higher memory footprint (~200MB vs ~20MB)
- Learning curve for operations team
- Debugging complexity for SSE streams

### What Caddy Provides:
- Automatic HTTPS via Let's Encrypt (zero config)
- HTTP/2 out of the box
- Simple, readable configuration
- Native SSE support with `flush_interval -1`
- 20MB memory footprint
- Single binary, easy updates

### When to Consider Envoy:
- Multiple backend services requiring service mesh
- gRPC transcoding needs
- Complex traffic shaping (canary, shadow)
- Multi-cluster deployments
- Advanced rate limiting beyond per-IP

**Recommendation:** Start with Caddy. Add Envoy when you have 5+ microservices.

---

## Quick Start (15 minutes)

### 1. Provision Vultr VPS

- **Plan:** High Performance, 2 vCPU, 4GB RAM ($24/month)
- **OS:** Ubuntu 24.04 LTS
- **Location:** Choose nearest to users
- **Features:** Enable IPv6, disable auto-backups (we do our own)

### 2. Bootstrap Server

SSH in as root and run:

```bash
# Download and run bootstrap script
curl -sSL https://raw.githubusercontent.com/YOUR_ORG/conversational-platform/main/deploy/scripts/bootstrap-vultr.sh | sudo bash
```

Or manually:

```bash
git clone https://github.com/YOUR_ORG/conversational-platform.git /tmp/platform
sudo bash /tmp/platform/deploy/scripts/bootstrap-vultr.sh
```

### 3. Configure Environment

```bash
# Switch to docker user
su - docker

# Create environment file
cd /opt/conversational-platform
cp config/.env.template config/.env
nano config/.env
```

**Required settings:**
```env
# Generate secure values!
NATS_TOKEN=$(openssl rand -base64 32)
JWT_SECRET=$(openssl rand -base64 32)

# Set your LLM provider
ANTHROPIC_API_KEY=sk-ant-...
# OR
OPENAI_API_KEY=sk-...
```

### 4. Configure Domain

```bash
# Update Caddyfile with your domain
sudo nano /etc/caddy/Caddyfile

# Replace api.yourdomain.com with your actual domain
# Replace admin@yourdomain.com with your email

# Reload Caddy
sudo systemctl reload caddy
```

### 5. Deploy

```bash
cd /opt/conversational-platform
./deploy.sh
```

### 6. Verify

```bash
# Check services
docker compose -f docker-compose.prod.yml ps

# Check health
curl https://api.yourdomain.com/health

# Check NATS
curl http://localhost:8222/healthz
```

---

## Security Hardening Checklist

### Completed by Bootstrap Script:
- [x] Non-root docker user created
- [x] SSH key-only authentication
- [x] Root SSH login disabled
- [x] UFW firewall configured
- [x] Fail2Ban protecting SSH
- [x] Automatic security updates
- [x] Kernel hardening (sysctl)

### Manual Steps Required:

#### 1. Restrict NATS Port
```bash
# Remove public NATS access
sudo ufw delete allow 4222/tcp

# Only allow from specific IPs (if needed externally)
sudo ufw allow from 10.0.0.0/8 to any port 4222 proto tcp
```

#### 2. Set Up Monitoring Alerts
```bash
# Install node_exporter for Prometheus
docker run -d \
  --name node-exporter \
  --restart unless-stopped \
  --net host \
  --pid host \
  -v /:/host:ro,rslave \
  quay.io/prometheus/node-exporter:latest \
  --path.rootfs=/host
```

#### 3. Configure Log Rotation
```bash
sudo nano /etc/logrotate.d/platform
```
```
/var/log/caddy/*.log {
    daily
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 caddy caddy
    sharedscripts
    postrotate
        systemctl reload caddy
    endscript
}
```

---

## Environment Variables Reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SERVER_PORT` | No | `8080` | API server port |
| `LOG_LEVEL` | No | `info` | Log level (debug/info/warn/error) |
| `NATS_URL` | Yes | - | NATS connection URL |
| `NATS_TOKEN` | Yes | - | NATS authentication token |
| `JWT_SECRET` | Yes | - | JWT signing secret (32+ chars) |
| `ANTHROPIC_API_KEY` | One required | - | Anthropic API key |
| `OPENAI_API_KEY` | One required | - | OpenAI API key |
| `RATE_LIMIT_REQUESTS` | No | `60` | Requests per window |
| `RATE_LIMIT_WINDOW` | No | `1m` | Rate limit window |
| `TRACING_ENABLED` | No | `false` | Enable OpenTelemetry |
| `TRACING_ENDPOINT` | No | - | OTLP endpoint URL |

---

## Operations Runbook

### View Logs
```bash
# API logs
./logs.sh api

# NATS logs
./logs.sh nats

# Caddy logs
sudo tail -f /var/log/caddy/api-access.log
```

### Restart Services
```bash
docker compose -f docker-compose.prod.yml restart api
docker compose -f docker-compose.prod.yml restart nats
```

### Update Application
```bash
./deploy.sh
```

### Backup NATS Data
```bash
./backup.sh
```

### Check Disk Usage
```bash
du -sh /var/lib/platform-data/*
docker system df
```

### Emergency: Clear NATS Stream
```bash
# WARNING: Deletes all messages!
docker exec -it nats nats stream purge CONVERSATIONS --force
```

---

## Scaling Guide

### Vertical Scaling (Recommended First)
1. Upgrade Vultr plan to 4 vCPU / 8GB RAM
2. Increase Docker memory limits in docker-compose

### Horizontal Scaling (When Needed)
1. Deploy NATS cluster (3 nodes minimum)
2. Add load balancer (Vultr Load Balancer)
3. Deploy multiple API instances
4. Consider Envoy for advanced routing

### Performance Tuning
```yaml
# docker-compose.prod.yml
services:
  api:
    deploy:
      resources:
        limits:
          memory: 1G
          cpus: '2'
    environment:
      - GOMAXPROCS=2
```

---

## Troubleshooting

### SSE Connections Dropping
```bash
# Check Caddy config has flush_interval
grep -A5 "reverse_proxy" /etc/caddy/Caddyfile

# Should have: flush_interval -1
```

### NATS Connection Refused
```bash
# Check NATS is running
docker compose -f docker-compose.prod.yml ps nats

# Check NATS logs
docker compose -f docker-compose.prod.yml logs nats

# Verify token matches
grep NATS_TOKEN config/.env
```

### High Memory Usage
```bash
# Check container stats
docker stats

# Clear Docker build cache
docker builder prune -af
```

### Certificate Issues
```bash
# Check Caddy logs
sudo journalctl -u caddy -f

# Force certificate renewal
sudo caddy reload --config /etc/caddy/Caddyfile
```

---

## Cost Estimate

| Component | Monthly Cost |
|-----------|--------------|
| Vultr VPS (2 vCPU, 4GB) | $24 |
| Domain name | ~$1 |
| Anthropic API (estimated) | Variable |
| **Total Infrastructure** | **~$25/month** |

---

## Support

- **GitHub Issues:** https://github.com/YOUR_ORG/conversational-platform/issues
- **Documentation:** This file
- **Monitoring:** Prometheus at `/metrics`
