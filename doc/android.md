# Run subs-check On Android

> Uses Termux.

## Prerequisites

- Make sure the network connection works.
- Android 7.0 or later is recommended.
- This guide assumes basic technical familiarity.

## Install Dependencies

```bash
pkg update && pkg add nodejs ca-certificates which proot termux-exec -y
```

## Download And Extract The Program

Handle this yourself according to your device and release package.

## Switch Environment

> Run this each time you open the terminal before starting subs-check.

```bash
# Give subs-check a more complete Linux environment.
termux-chroot

# If you encounter DNS issues, edit /etc/resolv.conf yourself.
echo "nameserver 223.5.5.5" > /etc/resolv.conf
```

## Set Environment Variables

```bash
# Temporary environment variables.
# Required on phones without root access. Rooted devices may not need this after granting permissions.
export SSL_CERT_FILE="/data/data/com.termux/files/usr/etc/tls/cert.pem"

export NODEBIN_PATH="$(which node)"
```

```bash
# Persistent environment variables. Reopen the terminal without setting them again.
echo 'export SSL_CERT_FILE="/data/data/com.termux/files/usr/etc/tls/cert.pem"' >> ~/.bashrc
echo 'export NODEBIN_PATH="$(which node)"' >> ~/.bashrc
source ~/.bashrc
```

## Run

```bash
./subs-check
```

## FAQ

1. If you get certificate errors, make sure `SSL_CERT_FILE` is set correctly.
2. If permissions are insufficient, run `chmod 755 subs-check`.
3. If node cannot be found, make sure `NODEBIN_PATH` is set correctly.
