#!/bin/bash
# init-streams.sh
# Run once to create JetStream streams

set -euo pipefail

NATS_URL="${NATS_URL:-nats://localhost:4222}"

echo "Connecting to NATS at $NATS_URL..."

# Create the conversations stream
nats stream add CONVERSATIONS \
  --server "$NATS_URL" \
  --subjects "conv.>" \
  --storage file \
  --replicas 1 \
  --retention limits \
  --max-age 365d \
  --max-bytes 100GB \
  --max-msg-size 8MB \
  --discard old \
  --dupe-window 2m \
  --deny-delete \
  --deny-purge \
  --compression s2 \
  --description "All conversation messages and events" \
  --defaults

echo "Stream created successfully!"

# Verify
echo ""
echo "Stream info:"
nats stream info CONVERSATIONS --server "$NATS_URL"
