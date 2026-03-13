#!/bin/bash
set -e

APP_DIR="/home/forge/myapp.com"
PREV=$(ls -t "$APP_DIR/releases" | sed -n '2p')
if [ -z "$PREV" ]; then
  echo "No previous release found"
  exit 1
fi
ln -sfn "$APP_DIR/releases/$PREV" "$APP_DIR/current"
sudo systemctl reload php8.2-fpm
echo "Rolled back to: $PREV"
