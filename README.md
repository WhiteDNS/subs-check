<h1 align="center">🚀 Subscription Check And Conversion Tool</h1>

<p align="center">
	<a href="https://github.com/beck-8/subs-check/releases"><img src="https://img.shields.io/github/v/release/beck-8/subs-check?style=flat-square&include_prereleases&label=version" /></a>
	<a href="https://github.com/beck-8/subs-check/releases"><img src="https://img.shields.io/github/downloads/beck-8/subs-check/total.svg?style=flat-square" /></a>
  <a href="https://hub.docker.com/r/beck8/subs-check/tags"><img src="https://img.shields.io/docker/pulls/beck8/subs-check" /></a>
	<a href="https://github.com/beck-8/subs-check/issues"><img src="https://img.shields.io/github/issues-raw/beck-8/subs-check.svg?style=flat-square&label=issues" /></a>
	<a href="https://github.com/beck-8/subs-check/graphs/contributors"><img src="https://img.shields.io/github/contributors/beck-8/subs-check?style=flat-square" /></a>
	<a href="https://github.com/beck-8/subs-check/blob/master/LICENSE"><img src="https://img.shields.io/github/license/beck-8/subs-check?style=flat-square" /></a>
</p>

---

> **✨ Fixes logic, simplifies operation, adds features, saves memory, and starts with one command without required configuration**

> **⚠️ Note:** Features are updated frequently. Check the latest [example config](https://github.com/beck-8/subs-check/blob/master/config/config.example.yaml) for current options.
> **⚠️ Note:** To review feature updates, see the [example config commit history](https://github.com/beck-8/subs-check/commits/master/config/config.example.yaml); config changes usually indicate feature or logic updates.

## 📸 Preview

![preview](./doc/images/preview.png)  
![result](./doc/images/results.png)  
![admin](./doc/images/admin.png)
| | |
|---|---|
| ![tgram](./doc/images/tgram.png) | ![dingtalk](./doc/images/dingtalk.png)  |

## ✨ Features

- **🔗 Subscription merging**
- **🔍 Node availability checks**
- **🗑️ Node deduplication**
- **⏱️ Node speed tests**
- **🎬 Streaming platform unlock checks**
- **✏️ Node renaming**
- **🔄 Subscription conversion across formats**
- **🔔 100+ notification channels**
- **🌐 Built-in Sub-Store**
- **🖥️ Web control panel**
- **⏰ Crontab expression support**
- **🖥️ Multi-platform support**

## 🛠️ Deployment And Usage

The first run generates a default config file in the current directory.

### 🚀 One-Click Install (Linux)

```bash
# Default install
bash <(curl -fsSL https://raw.githubusercontent.com/beck-8/subs-check/master/install.sh)

# Use wget
bash <(wget -qO- https://raw.githubusercontent.com/beck-8/subs-check/master/install.sh)

# Use a proxy if GitHub is unreachable
bash <(curl -fsSL https://ghfast.top/https://raw.githubusercontent.com/beck-8/subs-check/master/install.sh) https://ghfast.top/

# Alpine or other environments without bash
wget -qO /tmp/install.sh https://raw.githubusercontent.com/beck-8/subs-check/master/install.sh && sh /tmp/install.sh && rm -f /tmp/install.sh
```

<details>
  <summary>Script Details</summary>

The install script automatically:
1. Detects system architecture: x86_64, aarch64, armv7, or i386.
2. Downloads the latest version from GitHub Releases.
3. Installs to `/opt/subs-check`.
4. Configures the systemd service.
5. Asks whether to enable startup on boot.
6. Asks whether to start the service immediately.

**Service management:**
```bash
systemctl start subs-check    # Start
systemctl stop subs-check     # Stop
systemctl restart subs-check  # Restart
systemctl status subs-check   # Status
journalctl -u subs-check -f   # Logs
```

**Uninstall:**
```bash
systemctl stop subs-check
systemctl disable subs-check
rm -rf /opt/subs-check /etc/systemd/system/subs-check.service
systemctl daemon-reload
```

</details>

### 🪜 Proxy Settings (Optional)

<details>
  <summary>Show Details</summary>

If fetching non-GitHub subscriptions is slow, use standard `HTTP_PROXY` and `HTTPS_PROXY` environment variables. These variables do not affect node test speed.

```bash
# HTTP proxy example
export HTTP_PROXY=http://username:password@192.168.1.1:7890
export HTTPS_PROXY=http://username:password@192.168.1.1:7890

# SOCKS5 proxy example
export HTTP_PROXY=socks5://username:password@192.168.1.1:7890
export HTTPS_PROXY=socks5://username:password@192.168.1.1:7890

# SOCKS5H proxy example
export HTTP_PROXY=socks5h://username:password@192.168.1.1:7890
export HTTPS_PROXY=socks5h://username:password@192.168.1.1:7890
```

To accelerate GitHub links, use a public GitHub proxy or deploy your own acceleration with the `worker.js` shown in the self-hosted speed-test section.

```yaml
# GitHub proxy used for fetching subscriptions. Must end with /.
# github-proxy: "https://ghfast.top/"
github-proxy: "https://custom-domain/raw/"
```

</details>

### 🌐 Self-Hosted Speed-Test URL (Optional)

<details>
  <summary>Show Details</summary>

> **⚠️ Note:** Avoid Speedtest and Cloudflare download links because some nodes block speed-test sites.

1. Deploy [worker.js](./doc/cloudflare/worker.js) to Cloudflare Workers.
2. Bind a custom domain to reduce blocking by nodes.
3. Set `speed-test-url` in the config to your Workers URL:

```yaml
# 100MB
speed-test-url: https://custom-domain/speedtest?bytes=104857600
# 1GB
speed-test-url: https://custom-domain/speedtest?bytes=1073741824
```

</details>

### 🐳 Docker Run

> **⚠️ Note:**
> - Use `--memory="500m"` to limit memory.
> - Set the web control panel API key with the `API_KEY` environment variable.

```bash
# Basic run
docker run -d \
  --name subs-check \
  -p 8299:8299 \
  -p 8199:8199 \
  -v ./config:/app/config \
  -v ./output:/app/output \
  --restart always \
  ghcr.io/beck-8/subs-check:latest

# Run with proxy
docker run -d \
  --name subs-check \
  -p 8299:8299 \
  -p 8199:8199 \
  -e HTTP_PROXY=http://192.168.1.1:7890 \
  -e HTTPS_PROXY=http://192.168.1.1:7890 \
  -v ./config:/app/config \
  -v ./output:/app/output \
  --restart always \
  ghcr.io/beck-8/subs-check:latest
```

### 📜 Docker Compose

```yaml
version: "3"
services:
  subs-check:
    image: ghcr.io/beck-8/subs-check:latest
    container_name: subs-check
    volumes:
      - ./config:/app/config
      - ./output:/app/output
    ports:
      - "8299:8299"
      - "8199:8199"
    environment:
      - TZ=Asia/Shanghai
      # - HTTP_PROXY=http://192.168.1.1:7890
      # - HTTPS_PROXY=http://192.168.1.1:7890
      # - API_KEY=subs-check
    restart: always
    network_mode: bridge
```

### 📦 Binary Run

Download the matching version from [Releases](https://github.com/beck-8/subs-check/releases), extract it, and run it directly.

### 🖥️ Source Run

```bash
go run . -f ./config/config.yaml
```

## 🔔 Notification Channels (Optional)

<details>
  <summary>Show Details</summary>

> **📦 Supports 100+ notification channels** through [Apprise](https://github.com/caronc/apprise).

### 🌐 Vercel Deployment

1. Click [**here**](https://vercel.com/new/clone?repository-url=https://github.com/beck-8/apprise_vercel) to deploy Apprise.
2. After deployment, get the API URL, such as `https://testapprise-beck8s-projects.vercel.app/notify`.
3. A custom Vercel domain such as `diydomain.com` is recommended if access to Vercel is restricted in your region.

### 🐳 Docker Deployment

> **⚠️ Note:** arm/v7 is not supported.

```bash
# Basic run
docker run --name apprise -p 8000:8000 --restart always -d caronc/apprise:latest

# Run with proxy
docker run --name apprise \
  -p 8000:8000 \
  -e HTTP_PROXY=http://192.168.1.1:7890 \
  -e HTTPS_PROXY=http://192.168.1.1:7890 \
  --restart always \
  -d caronc/apprise:latest
```

### 📝 Notification Config

```yaml
# Apprise API server URL.
# https://notify.xxxx.us.kg/notify
apprise-api-server: "https://diydomain.com/notify"
# Notification targets.
# Supports 100+ notification channels. See https://github.com/caronc/apprise for formats.
recipient-url: 
  # Telegram format: tgram://{bot_token}/{chat_id}
  # - tgram://xxxxxx/-1002149239223
  # DingTalk format: dingtalk://{Secret}@{ApiKey}
  # - dingtalk://xxxxxx@xxxxxxx
# Custom notification title.
notify-title: "🔔 Node Status Update"
```
</details>

## 💾 Save Method Config

> **⚠️ Note:** Change the `save-method` config when selecting a save method.

- **Local save**: save to the `./output` folder.
- **R2**: save to Cloudflare R2. See [configuration](./doc/r2.md).
- **Gist**: save to GitHub Gist. See [configuration](./doc/gist.md).
- **WebDAV**: save to a WebDAV server. See [configuration](./doc/webdav.md).
- **S3**: save to S3 object storage.

## 📲 Subscription Usage

> **💡 Tip:** Built-in Sub-Store can generate multiple subscription formats. Advanced users can build many custom flows.

**🚀 General Subscriptions**
```bash
# General subscription
http://127.0.0.1:8299/download/sub

# URI subscription
http://127.0.0.1:8299/download/sub?target=URI

# Mihomo/ClashMeta
http://127.0.0.1:8299/download/sub?target=ClashMeta

# Clash
http://127.0.0.1:8299/download/sub?target=Clash

# V2Ray
http://127.0.0.1:8299/download/sub?target=V2Ray

# ShadowRocket
http://127.0.0.1:8299/download/sub?target=ShadowRocket

# Quantumult
http://127.0.0.1:8299/download/sub?target=QX

# Sing-Box
http://127.0.0.1:8299/download/sub?target=sing-box

# Surge
http://127.0.0.1:8299/download/sub?target=Surge

# Surfboard
http://127.0.0.1:8299/download/sub?target=Surfboard
```

**🚀 Mihomo/Clash Subscription With Rules**

The default override is `https://raw.githubusercontent.com/beck-8/override-hub/refs/heads/main/yaml/ACL4SSR_Online_Full.yaml`.
You can change it with `mihomo-overwrite-url`.

```bash
http://127.0.0.1:8299/api/file/mihomo
```

## 🌐 Built-In Ports

After a check, subs-check saves three files into the output directory. All files in output are served on port 8199.

| Service URL | Format | Source |
|---|---|---|
| `http://127.0.0.1:8199/sub/all.yaml` | Clash-format nodes | Generated directly by subs-check |
| `http://127.0.0.1:8199/sub/mihomo.yaml` | Mihomo/Clash subscription with routing rules | Converted by sub-store above, then served |
| `http://127.0.0.1:8199/sub/base64.txt` | Base64 subscription | Converted by sub-store above, then served |

## 🗺️ Architecture

<details>
  <summary>Show Details</summary>

```mermaid
graph TD
    A[Subscription URLs] -->|Fetch subscriptions| B[subs-check]
    subgraph subs-check pipeline
        B -->|Convert to YAML| B1[Node deduplication]
        B1 -->|Remove redundant nodes| B2[Alive check]
        B2 -->|Node usable| B3[Media checks + rename]
        B2 -->|Node unusable| X[Discard]
        B3 -->|filter passed| B4[Speed test]
        B3 -->|filter failed| X[Discard]
        B4 -->|Speed passes| B5[Generate all.yaml]
        B4 -->|Speed fails| X[Discard]
    end
    B5 -->|Save to output directory| C[output directory]
    B5 -->|Upload all.yaml| D[sub-store]
    C -->|Save to destinations| H1[R2/Gist/WebDAV/S3]
    H1 -->|Storage complete| H2[Send notification]
    D -->|Provide conversion service| E[sub-store conversion service]
    subgraph sub-store standalone features
        E -->|Generate config files| E1[mihomo.yaml, base64.txt]
        E -->|Convert other formats| E2[Clash, V2Ray, ShadowRocket, etc.]
        E -->|Subscription sharing| E3[Share subscription links]
    end
    E1 -->|Save to output directory| C
    C -->|File service| F[8199 port: /sub]
    B -->|Web management| G[8199 port: /admin]
``` 

</details>

## 🙏 Credits

[cmliu](https://github.com/cmliu), [Sub-Store](https://github.com/sub-store-org/Sub-Store), [bestruirui](https://github.com/bestruirui/BestSub), [1password](https://1password.com/), [ipinfo.io](https://ipinfo.io/)

## ⭐ Star History

[![Stargazers over time](https://starchart.cc/beck-8/subs-check.svg?variant=adaptive)](https://starchart.cc/beck-8/subs-check)

## ⚖️ Disclaimer

This tool is provided only for learning and research. Users are responsible for their own risk and must comply with applicable laws and regulations.
