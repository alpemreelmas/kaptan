# Kaptan — Genel Bakış

Kaptan, Forge ile yönetilen VPS sunucularına merkezi mikroservis deploy aracıdır. SSH tabanlı deploy'ların yerini mTLS ile kimlik doğrulanmış gRPC kanalı alır. Deploy mantığı tamamen her projenin kendi repo'sundaki `.kaptan/deploy.sh` dosyasında yaşar — kaptan sadece scripti çalıştırır ve çıktıyı geri akıtır.

---

## Bileşenler

| Bileşen | Binary | Nerede Çalışır | Görev |
|---------|--------|---------------|-------|
| **kaptan** | `bin/kaptan` | Geliştirici makinesi | CLI — komutları gönderir, çıktıyı gösterir |
| **reis** | `bin/reis` | VPS sunucusu | gRPC sunucusu — scriptleri çalıştırır, logları akıtır |

---

## Mimari

```
┌──────────────────────────────────────────────────────────┐
│  Geliştirici makinesi                                     │
│                                                           │
│   kaptan (CLI)  ──mTLS/gRPC──►  reis (VPS)               │
│       │                             │                     │
│       │  DeployRequest              │  .kaptan/deploy.sh  │
│       │  RollbackRequest            │  .kaptan/rollback.sh│
│       │  StatusRequest              │  health check       │
│       │  LogRequest                 │  systemd/app logları│
│       │  GraphRequest               │  nginx log parser   │
│       │                             │                     │
│       ◄──── streaming ExecEvents ───┘                     │
└──────────────────────────────────────────────────────────┘
```

**Temel prensipler:**
- Deploy mantığı repoda kalır (`.kaptan/deploy.sh`), kaptan içinde değil.
- Tüm iletişim mTLS ile kimlik doğrulanmış gRPC üzerinden — çalışma zamanında SSH gerekmez.
- `reis` tek bir durumsuz binary; veritabanı veya durum dosyası yoktur.

---

## mTLS Sertifika Modeli

Kaptan mutual TLS kullanır — hem istemci (kaptan) hem sunucu (reis) aynı CA tarafından imzalanmış sertifikalarla kimliğini kanıtlar.

```
CA (ca.crt / ca.key)
  ├── imzalar → server.crt  (VPS'te, reis tarafından kullanılır)
  └── imzalar → client.crt  (geliştirici makinesinde, kaptan tarafından kullanılır)
```

`ca.key` hiçbir zaman makinenizden çıkmak zorunda değildir. `ca.crt` ise her iki tarafta da bulunması gereken tek dosyadır.

---

## Hızlı Başlangıç (uçtan uca)

```bash
# 1. CLI'ı derle ve PATH'e ekle
go build -o ~/bin/kaptan ./cli

# 2. mTLS sertifikalarını oluştur
kaptan cert init

# 3. VPS'e reis'i kur (ca.crt'yi kopyalar, binary'yi kurar)
kaptan server bootstrap web-prod-1 deploy@1.2.3.4

# 4. Sunucuyu global config'e kaydet
kaptan server add web-prod-1 1.2.3.4:7000

# 5. Proje repo'sunda .kaptan/config.yaml oluştur
cat > .kaptan/config.yaml <<EOF
service: my-api
server:  web-prod-1
path:    /srv/my-api
health_url: http://localhost:8080/healthz
EOF

# 6. .kaptan/deploy.sh oluştur
cat > .kaptan/deploy.sh <<'EOF'
#!/usr/bin/env bash
set -e
echo "[1/3] Pull latest image"
docker pull myrepo/my-api:latest

echo "[2/3] Run migrations"
docker run --rm myrepo/my-api:latest ./migrate

echo "[3/3] Restart service"
systemctl restart my-api
EOF
chmod +x .kaptan/deploy.sh

# 7. Deploy et
kaptan deploy

# Tüm prod sunucularda sağlık durumunu gör
kaptan status --tag=prod

# Log akıt
kaptan logs --tail=100

# Bağımlılık grafiğini görüntüle
kaptan graph

# Gerekirse geri al
kaptan rollback
```

---

## Deploy Script Protokolü

`reis`, `project_path` içinde `.kaptan/deploy.sh` (ve `.kaptan/rollback.sh`) çalıştırır. Scriptler şunları alır:
- Çalışma dizini `project_path` olarak ayarlanmış
- Stdout ve stderr satır satır CLI'a geri akıtılır

**Faz bildirimi** (opsiyonel ama önerilir):
```bash
echo "[N/M] Faz açıklaması"
```
TUI bu deseni ayrıştırarak bir ilerleme listesi gösterir. Faz satırı içermeyen düz scriptler de sorunsuz çalışır — çıktı log panelinde gösterilir.

**Çıkış kodları:**
- `0` — başarı (sağlık kontrolü sırada çalışır)
- sıfır dışı — deploy başarısız, otomatik rollback tetiklenmez (rollback yalnızca başarısız sağlık kontrolü tarafından tetiklenir)

---

## Kaynaklardan Derleme

Gereksinimler: Go 1.22+, `buf` (yalnızca proto yenileme için)

```bash
git clone https://github.com/alpemreelmas/kaptan
cd kaptan

# Her şeyi derle
make build
# Çıktı: bin/kaptan  bin/reis

# Yalnızca CLI
make cli

# Yalnızca agent
make agent

# (Opsiyonel) Protobuf'u yenile
cd proto && buf generate
```

Go workspace yapısı (`go.work`), üç modülü bir arada tutar:

| Modül | Yol |
|-------|-----|
| `github.com/alpemreelmas/kaptan/proto` | `proto/gen/` |
| `github.com/alpemreelmas/kaptan/agent` | `agent/` |
| `github.com/alpemreelmas/kaptan/cli` | `cli/` |

---

## Daha Fazla Bilgi

- [kaptan.md](kaptan.md) — CLI komutları, konfigürasyon, TUI ekranları
- [reis.md](reis.md) — Agent kurulumu, konfigürasyon, gRPC API
