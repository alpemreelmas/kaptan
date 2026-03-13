#!/bin/bash
set -euo pipefail

APP_DIR="/home/forge/myapp.com"
RELEASE="$APP_DIR/releases/$(date +%Y%m%d-%H%M%S)-$(git -C "$APP_DIR/repo" rev-parse --short HEAD)"

echo "[1/5] git pull..."
git -C "$APP_DIR/repo" pull origin main

echo "[2/5] composer install..."
cp -r "$APP_DIR/repo" "$RELEASE"
composer install --no-dev --working-dir="$RELEASE"

echo "[3/5] symlink..."
ln -sfn "$RELEASE" "$APP_DIR/current"

echo "[4/5] restart..."
php "$APP_DIR/current/artisan" queue:restart
sudo systemctl reload php8.2-fpm

echo "[5/5] done."
