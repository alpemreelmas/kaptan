# reis — Agent Referansı

`reis`, her VPS sunucusunda systemd servisi olarak çalışan gRPC sunucusudur. `kaptan` CLI'dan gelen komutları alır; deploy scriptlerini çalıştırır, çıktıyı akıtır, sağlık kontrolü yapar ve gerektiğinde otomatik rollback başlatır.

---

## Kurulum

`kaptan server bootstrap` komutu kurulum sürecini otomatikleştirir. Manuel kurulum için `install.sh` scripti kullanılabilir:

```bash
curl -fsSL https://raw.githubusercontent.com/alpemreelmas/kaptan/main/install.sh | bash
```

Script şunları yapar:
1. En son `reis` binary'sini GitHub Releases'ten indirir
2. `~/.reis/bin/reis` konumuna kurar
3. `~/.reis/config.yaml` varsayılan konfigürasyonunu oluşturur
4. `~/.reis/certs/` sertifika dizinini hazırlar
5. systemd servisini yazar ve başlatır (`systemctl` mevcutsa)

---

## Konfigürasyon

### Konfigürasyon Dosyası — `~/.reis/config.yaml`

```yaml
listen_addr: ":7000"
tls:
  cert: ~/.reis/certs/server.crt   # sunucu sertifikası
  key:  ~/.reis/certs/server.key   # sunucu özel anahtarı
  ca:   ~/.reis/certs/ca.crt       # istemcileri doğrulamak için CA sertifikası
```

| Alan | Varsayılan | Açıklama |
|------|-----------|----------|
| `listen_addr` | `:7000` | Dinlenecek TCP adresi |
| `tls.cert` | — | Sunucu TLS sertifikası yolu |
| `tls.key` | — | Sunucu TLS özel anahtarı yolu |
| `tls.ca` | — | İstemci sertifikalarını doğrulamak için CA sertifikası |

> `tls` bölümü atlanırsa veya herhangi bir yol boş bırakılırsa, `reis` **TLS olmadan** başlar (yalnızca geliştirme — log'a uyarı yazılır).

---

### Başlatma ve Bayraklar

```
reis [bayraklar]
```

| Bayrak | Varsayılan | Açıklama |
|--------|-----------|----------|
| `--config` | `~/.reis/config.yaml` | Konfigürasyon dosyası yolu |

Konfigürasyon dosyası bulunamazsa `reis` varsayılanlarla (TLS'siz, `:7000`) başlar.

Yapılandırılmış günlükleme `log/slog` ile JSON uyumlu çıktı üretir:

```
INFO  reis starting addr=:7000
INFO  mTLS enabled
INFO  listening addr=:7000
```

---

### systemd Servisi

`install.sh` tarafından oluşturulan unit dosyası:

```ini
[Unit]
Description=reis gRPC deployment agent
After=network.target

[Service]
Type=simple
User=deploy
ExecStart=/home/deploy/.reis/bin/reis --config /home/deploy/.reis/config.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
# Servis yönetimi
systemctl status reis
systemctl restart reis
journalctl -u reis -f
```

---

## Sertifika Kurulumu (Sunucu Tarafı)

`kaptan server bootstrap` bu adımları otomatik yapar. Manuel kurulum:

```bash
# 1. Geliştirici makinesinden CA sertifikasını kopyala
scp ~/.kaptan/certs/ca.crt deploy@sunucu:~/.reis/certs/ca.crt

# 2. Sunucuda sunucu sertifikası oluştur (CA tarafından imzalanmış)
#    ya da kaptan cert init sonrası oluşturulan server.crt/server.key'i kopyala

# 3. reis'i yeniden başlat
systemctl restart reis
```

Sertifika dosyaları yapısı:
```
~/.reis/
  config.yaml
  certs/
    ca.crt       # kaptan'dan kopyalanan CA sertifikası
    server.crt   # bu sunucunun sertifikası
    server.key   # bu sunucunun özel anahtarı
```

---

## gRPC API

Servis: `agent.v1.AgentService`
Varsayılan port: `:7000`

---

### `Deploy` (sunucu-akış)

`project_path` içinde belirtilen scripti çalıştırır ve çıktıyı satır satır akıtır.

**İstek:**

| Alan | Tür | Açıklama |
|------|-----|----------|
| `project_path` | string | Sunucudaki projenin mutlak yolu |
| `script` | string | Uzantısız script adı (varsayılan: `"deploy"`) |
| `dry_run` | bool | true ise çalıştırmadan ne yapacağını gösterir |

**Akış olayları (`ExecEvent`):**

| Alan | Tür | Açıklama |
|------|-----|----------|
| `line` | string | Stdout/stderr'den bir satır |
| `is_stderr` | bool | Satır stderr'den geldiyse true |
| `done` | bool | Son olayda true |
| `exit_code` | int32 | Script çıkış kodu (yalnızca `done=true` olduğunda geçerli) |

**Deploy sonrası akış:**

Başarılı deploy (`exit_code=0`) sonrasında `reis` otomatik olarak sağlık kontrolü yapar:

```
deploy.sh çıkış kodu 0
    └─► GET health_url (30 sn timeout)
            ├─ 2xx  →  "[health] → 200 OK" akıtılır, deploy başarılı
            └─ diğer → rollback.sh otomatik tetiklenir
                        "[rollback] ..." log satırları akıtılır
                        gRPC status: codes.Internal döner
```

**Dry-run davranışı:** `reis` iki sentetik olay gönderir ve çıkar, dosya sistemine dokunmaz:
```
[dry-run] would execute: /yol/.kaptan/deploy.sh
[dry-run] done
```

---

### `Rollback` (sunucu-akış)

`project_path` içinde `.kaptan/rollback.sh`'i çalıştırır. Akış olayları `Deploy` ile aynıdır.

---

### `GetStatus` (tekli)

`reis`'in bildiği tüm servislerin sağlık durumunu döner.

**`ServiceStatus` alanları:**

| Alan | Tür | Açıklama |
|------|-----|----------|
| `service_name` | string | Servis adı |
| `healthy` | bool | Sağlık URL'si 2xx döndürdüyse true |
| `status_code` | int32 | Son sağlık kontrolündeki HTTP durum kodu |

---

### `StreamLogs` (sunucu-akış)

Uzak bir log dosyasını tail eder ve satırları akıtır.

**İstek:**

| Alan | Tür | Açıklama |
|------|-----|----------|
| `project_path` | string | Varsayılan log dosyasını bulmak için kullanılır |
| `log_file` | string | Açık log dosyası yolu (otomatik algılamayı geçersiz kılar) |
| `tail` | int32 | Başlangıçta okunacak son satır sayısı (varsayılan: 50) |

---

### `GetDependencyGraph` (tekli)

Nginx erişim loglarını ayrıştırarak servis bağımlılık grafiği döner.

**İstek:**

| Alan | Tür | Açıklama |
|------|-----|----------|
| `log_file` | string | Nginx log yolu (varsayılan: `/var/log/nginx/access.log`) |
| `internal_domains` | []string | Internal kenar sınıflandırması için glob desenleri (ör. `*.internal`) |

**Yanıt — `GraphEdge` listesi:**

| Alan | Tür | Açıklama |
|------|-----|----------|
| `from` | string | Kaynak servis (log dosyası adından türetilir) |
| `to` | string | Hedef host |
| `status_code` | int32 | Gözlemlenen HTTP durumu |
| `error_count` | int32 | 4xx/5xx yanıt sayısı |
| `kind` | enum | `INTERNAL` veya `EXTERNAL` |

---

## Bağımlılık Grafiği Ayrıştırıcısı

`reis`, nginx erişim logu satırlarını aşağıdaki desenle eşleştirir:

```
"GET http://servis-adi:3000/yol HTTP/1.1" 200 1234
```

- **Kaynak**, log dosyası adından çıkarılır: `/var/log/nginx/my-api.access.log` → `my-api`
- **Hedef**, istek URL'sinden çıkarılan upstream host'tur
- Kenarlar `(kaynak, hedef, durum_kodu)` üçlüsüne göre tekilleştirilir ve gruplandırılır
- Hedef host `internal_domains` desenlerinden herhangi biriyle eşleşiyorsa veya nokta içermiyorsa (ör. `postgres`) `INTERNAL` olarak sınıflandırılır
- `error_count`, durum kodu ≥ 400 olan yanıt sayısıdır

---

## Sağlık Kontrolü ve Otomatik Rollback

`reis`, her başarılı deploy sonrasında `project_path` içindeki `.kaptan/config.yaml` dosyasını okur:

```yaml
# proje içindeki .kaptan/config.yaml (sunucu tarafı)
service:    my-api
health_url: http://localhost:8080/healthz
```

`health_url` tanımlıysa:
- 30 saniyelik timeout ile HTTP GET gönderilir
- 2xx → `[health] → 200 OK` log satırı akıtılır, deploy başarılı sayılır
- Diğer → `.kaptan/rollback.sh` otomatik çalıştırılır, `[rollback] ...` satırları akıtılır, `codes.Internal` ile döner

`health_url` tanımlı değilse sağlık kontrolü atlanır ve deploy başarılı sayılır.
