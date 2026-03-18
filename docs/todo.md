# Kaptan + Reis TODO

## Güvenlik

- [ ] **Port 7000 IP whitelist** — Reis agent şu an herkese açık. UFW'de sadece kaptan'ın çalıştığı IP'ye izin verilmeli:
  ```bash
  ufw allow from <dev-machine-ip> to any port 7000
  ufw delete allow 7000/tcp
  ```

- [ ] **GitHub Actions workflow scope** — Release workflow push edilemedi, PAT'a `workflow` scope eklenmeli (`gh auth refresh -s workflow`)

- [ ] **Reis server cert rotation** — Mevcut server cert 10 yıl geçerli (3650 gün). Üretim için 1 yıl + otomatik rotation mekanizması eklenmeli.

- [ ] **Client cert revocation** — Kaptan'ın mTLS'i cert revocation (CRL/OCSP) desteklemiyor. Ele geçirilen bir client cert sonsuza kadar geçerli. `kaptan cert rotate` komutu var ama otomatik değil.

- [ ] **Reis config dosyası izinleri** — `/root/.reis/config.yaml` ve `/etc/server.key` izinleri `600` olmalı:
  ```bash
  chmod 600 /root/.reis/config.yaml /etc/server.key
  ```

- [ ] **Bootstrap SCP güvenliği** — `kaptan server bootstrap` çalışmıyor (GitHub Releases yok). Binary'ler GitHub Actions ile release edilince bu otomatik düzelecek. O zamana kadar manuel SCP zorunlu.

## Eksik Özellikler

- [ ] **GitHub Actions release workflow** — `.github/workflows/release.yml` oluşturuldu ama push edilemedi. `gh auth refresh -s workflow` ile PAT güncellendikten sonra `git push origin main && git push origin v0.0.1` çalıştırılmalı.

- [ ] **`kaptan server list`** — Server listesini gösteren komut yok, sadece `config.yaml`'a bakarak anlaşılıyor.

- [ ] **`kaptan deploy` TUI — non-interactive ortam** — CI/CD veya SSH pipe'ında `--no-tui` flag'i zorunlu. Varsayılan davranış `--no-tui` olmalı ya da TTY yoksa otomatik algılanmalı.

- [ ] **Rollback testi** — `rollback.sh` yazıldı ve `detached HEAD` bug'ı düzeltildi ama gerçek bir rollback senaryosunda test edilmedi.

- [ ] **Health check retry** — Kaptan health check'i deploy sonrası tek seferinde yapıyor. Uygulama yavaş başlarsa false-negative rollback tetikleniyor. Retry sayısı ve interval konfigüre edilebilir olmalı.

- [ ] **Multi-server deploy** — `--all` ve `--tag` flag'leri var ama test edilmedi.

## İyileştirmeler

- [ ] **deploy.sh — dist commit etmek** — Her deploy'da `npm ci --include=dev` + `tsc` çalıştırılıyor (~10s). `dist/` klasörü git'e commit edilirse deploy adımı sadece `git pull + pm2 restart` olur.

- [ ] **Reis binary path** — Şu an `/usr/local/bin/reis`. `install.sh` `~/.reis/bin/reis` bekliyor. Tutarsızlık var, ikisi hizalanmalı.

- [ ] **Systemd reis servisi User=root** — Reis root olarak çalışıyor. Dedicated `reis` user oluşturulup least-privilege ile çalıştırılmalı.
