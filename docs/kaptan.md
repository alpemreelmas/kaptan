# kaptan — CLI Referansı

`kaptan`, geliştirici makinesinde çalışan komut satırı aracıdır. VPS'teki `reis` agent'ına mTLS/gRPC üzerinden bağlanarak deploy, rollback, log ve izleme komutlarını iletir.

---

## Konfigürasyon

### Global Konfigürasyon — `~/.kaptan/config.yaml`

Tüm bilinen sunucuları ve opsiyonel graf ayarlarını tanımlar. Bu dosya `kaptan server add` komutuyla otomatik oluşturulur veya elle düzenlenebilir.

```yaml
servers:
  - name: web-prod-1
    host: "1.2.3.4:7000"
    tags: ["prod", "web"]
    tls:
      cert: ~/.kaptan/certs/client.crt
      key:  ~/.kaptan/certs/client.key
      ca:   ~/.kaptan/certs/ca.crt

  - name: web-staging-1
    host: "5.6.7.8:7000"
    tags: ["staging"]
    tls:
      cert: ~/.kaptan/certs/client.crt
      key:  ~/.kaptan/certs/client.key
      ca:   ~/.kaptan/certs/ca.crt

graph:
  internal_domains:
    - "*.internal"
    - "*.svc.cluster.local"
    - "localhost"
```

| Alan | Açıklama |
|------|----------|
| `servers[].name` | Komutlarda kullanılan sunucu takma adı |
| `servers[].host` | `host:port` formatında adres |
| `servers[].tags` | `--tag` filtresi için etiketler |
| `servers[].tls` | mTLS sertifika yolları; boş bırakılırsa TLS'siz bağlanır (yalnızca geliştirme) |
| `graph.internal_domains` | Bu desenlere uyan host'lar bağımlılık grafında "internal" olarak işaretlenir |

---

### Proje Konfigürasyonu — `.kaptan/config.yaml`

Her proje repo'sunun kökünde bulunur. `kaptan deploy` çalıştırıldığında bu dosyadan okur.

```yaml
service: my-api                          # servis adı
server:  web-prod-1                      # global config'deki sunucu adı
path:    /srv/my-api                     # sunucu üzerindeki proje yolu
health_url: http://localhost:8080/healthz  # deploy sonrası kontrol edilir
```

| Alan | Zorunlu | Açıklama |
|------|---------|----------|
| `service` | evet | İnsan tarafından okunabilir servis adı |
| `server` | evet | `~/.kaptan/config.yaml`'daki sunucu adı |
| `path` | evet | Sunucu üzerindeki mutlak proje yolu |
| `health_url` | hayır | Deploy başarılıysa `reis` bu URL'yi kontrol eder |

---

## Komutlar

### `kaptan deploy`

Yapılandırılmış sunucuda `.kaptan/deploy.sh`'i çalıştırır ve çıktıyı akıtır.

```
kaptan deploy [bayraklar]
```

| Bayrak | Varsayılan | Açıklama |
|--------|-----------|----------|
| `--server <ad>` | `.kaptan/config.yaml`'dan | Hedef sunucuyu geçersiz kıl |
| `--dry-run` | false | Çalıştırmadan önizle |
| `--all` | false | `--tag` ile eşleşen tüm sunuculara deploy et |
| `--tag <etiket>` | — | `--all` ile birlikte sunucu filtresi |
| `--no-tui` | false | TUI yerine düz metin çıktısı |

**Yeniden deneme:** Bağlantı hatalarında üstel geri çekilme ile 3'e kadar deneme.

**Paralel deploy:** `--all --tag=prod` her eşleşen sunucu için ayrı bir goroutine başlatır ve tüm hataları toplar.

```bash
kaptan deploy                            # TUI çıktısıyla deploy
kaptan deploy --no-tui                   # düz metin akışı
kaptan deploy --dry-run                  # yalnızca önizle
kaptan deploy --server web-staging-1    # sunucuyu geçersiz kıl
kaptan deploy --all --tag=prod          # tüm prod sunucularına paralel deploy
```

**TUI ekranı:**

Deploy TUI varsayılan olarak aktiftir (`--no-tui` ile devre dışı bırakılır). Deploy scriptinizden gelen `[N/M] açıklama` satırlarını ayrıştırarak fazları izler.

```
╭─────────────────────────────────────────╮
│ kaptan deploy                           │
│                                         │
│  Service    my-api                      │
│  Server     web-prod-1 (1.2.3.4:7000)  │
│  Script     deploy                      │
│                                         │
│  [1/3] Pull latest image          ✓    │
│  [2/3] Run migrations             ●    │  ← çalışıyor
│  [3/3] Restart service            ·    │  ← bekliyor
│                                         │
│  ─── log ───                           │
│  Pulling from registry...              │
╰─────────────────────────────────────────╯
```

Faz durum simgeleri: `✓` tamamlandı · `●` çalışıyor · `✗` başarısız · `·` bekliyor

---

### `kaptan rollback`

Sunucuda `.kaptan/rollback.sh`'i çalıştırır. Çıktı düz metin olarak akıtılır.

```
kaptan rollback [--server <ad>]
```

| Bayrak | Varsayılan | Açıklama |
|--------|-----------|----------|
| `--server <ad>` | `.kaptan/config.yaml`'dan | Hedef sunucuyu geçersiz kıl |

---

### `kaptan status`

Yapılandırılmış tüm servislerin sağlık durumunu kontrol eder. Tüm sunucuları paralel sorgular ve TUI tablosu gösterir.

```
kaptan status [--tag <etiket>]
```

| Bayrak | Varsayılan | Açıklama |
|--------|-----------|----------|
| `--tag <etiket>` | — | Sunucuları etikete göre filtrele |

Çıktı sütunları: `Server`, `Service`, `Healthy` (✓/✗), `HTTP Status Code`.

---

### `kaptan logs`

Uzak servisten log akıtır.

```
kaptan logs [bayraklar]
```

| Bayrak | Varsayılan | Açıklama |
|--------|-----------|----------|
| `--server <ad>` | `.kaptan/config.yaml`'dan | Hedef sunucu |
| `--tail <n>` | 50 | Akışa başlanacak son satır sayısı |
| `--file <yol>` | otomatik | Sunucudaki açık log dosyası yolu |

---

### `kaptan graph`

Nginx erişim loglarından servis bağımlılık grafını çeker ve görüntüler.

```
kaptan graph [bayraklar]
```

| Bayrak | Varsayılan | Açıklama |
|--------|-----------|----------|
| `--server <ad>` | `.kaptan/config.yaml`'dan | Hedef sunucu |
| `--log-file <yol>` | `/var/log/nginx/access.log` | Sunucudaki erişim log yolu |

Etkileşimli TUI gösterir. Çıkmak için `q` veya `Ctrl+C`.

```
╭──────────────────────────────────────────────╮
│ kaptan graph — web-prod-1                    │
│                                              │
│ my-api                                       │
│     ├─[200]──► auth-service                  │
│     ├─[200]──► postgres.internal             │
│     └─[503]──► payment-api  ← 12 hata/5dak  │
│                                              │
│ (çıkmak için q)                              │
╰──────────────────────────────────────────────╯
```

Graf açıklaması:
- Yeşil `[2xx]` — başarılı upstream çağrısı
- Kırmızı `[4xx/5xx]` — hata yanıtı, 5 dakikalık pencere başına hata sayısıyla
- Internal kenarlar servis adlarıyla; external kenarlar tam host adıyla gösterilir

---

### `kaptan cert init`

mTLS için öz imzalı CA ve istemci sertifikası çifti oluşturur.

```
kaptan cert init
```

Oluşturulan dosyalar:
```
~/.kaptan/certs/
  ca.crt       # CA sertifikası (sunucuya kopyalanır)
  ca.key       # CA özel anahtarı (gizli tutulur)
  client.crt   # istemci sertifikası
  client.key   # istemci özel anahtarı
```

Algoritma: ECDSA P-256. Çalıştırdıktan sonra yazdırılan adımları izleyerek agent'ı bootstrap edin.

---

### `kaptan cert rotate`

Mevcut CA'yı kullanarak istemci sertifikasını yeniler.

```
kaptan cert rotate --server <ad>
```

| Bayrak | Zorunlu | Açıklama |
|--------|---------|----------|
| `--server <ad>` | evet | Sunucu adı (bilgilendirme amaçlı) |

`~/.kaptan/certs/client.{crt,key}` dosyalarının üzerine yazar. Rotasyondan sonra yeniden bağlanın.

---

### `kaptan server add`

`~/.kaptan/config.yaml`'a bir sunucu kaydeder.

```
kaptan server add <ad> <adres>
```

```bash
kaptan server add web-prod-1 1.2.3.4:7000
```

---

### `kaptan server bootstrap`

SSH üzerinden VPS'e `reis` binary'sini kurar, CA sertifikasını kopyalar ve agent konfigürasyonunu oluşturur.

```
kaptan server bootstrap <ad> <ssh-kullanıcı@host>
```

Yapılanlar:
1. Sunucuya SSH ile bağlanır
2. Uzak install.sh scriptini çalıştırır (`reis` binary'sini indirir)
3. `~/.kaptan/certs/ca.crt`'yi sunucudaki `~/.reis/certs/` dizinine kopyalar
4. `reis`'i systemd servisi olarak yeniden başlatır

```bash
kaptan server bootstrap web-prod-1 deploy@1.2.3.4
```

---

## Sertifika Kurulum Akışı

```bash
# Sertifikaları oluştur
kaptan cert init

# Agent'ı kur ve ca.crt'yi kopyala
kaptan server bootstrap web-prod-1 user@1.2.3.4

# Sunucuyu kaydet
kaptan server add web-prod-1 1.2.3.4:7000
```

**Rotasyon:**
```bash
kaptan cert rotate --server web-prod-1
# Eski istemci sertifikası reddedilir — yeniden bağlanın
```
