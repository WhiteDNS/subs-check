# sub-store Backend Usage Guide

> In older versions, opening `http://127.0.0.1:8299` redirected to `https://sub-store.vercel.app/subs`. Newer versions have changed.

> Do not panic.

> The program is working. sub-store is a frontend/backend separated project. When you directly access the backend, it redirects you to the public frontend page.

> Configure the backend address and you can operate your backend directly.

## Direct Access Redirects Here

> This page may not even open if your network is poor.

![Step 1](./images/sub-store1.png)

## Backend Settings In The Bottom-Right Settings

![Step 2](./images/sub-store2.png)

## Enter A Name And Backend Address, Then Save

![Step 3](./images/sub-store3.png)

## An Error Is Expected

This happens because browsers do not allow an HTTPS frontend to access an HTTP backend.

> **Chromium-based browser workaround:** other browsers usually have similar behavior; search for the equivalent workaround yourself.

> Solution from Xiaoyi: an HTTPS frontend cannot request a non-local HTTP backend, and some browsers also block local HTTP backends. Configure a reverse proxy or self-host an HTTP frontend on your LAN.

![](./images/sub-store7.png)
![](./images/sub-store8.png)
![](./images/sub-store9.png)

## Switch To The Backend You Added

![Step 4](./images/sub-store4.png)

## Subscription Management Page

> Rule-free subscriptions come from here.

![Step 5](./images/sub-store5.png)

## File Management Page

> The rule-based `mihomo.yaml` file comes from here.

![Step 6](./images/sub-store6.png)

## Want To DIY?

Create a new subscription or file based on the existing one. Do not modify the configuration reserved for subs-check.

## Security / Custom PATH

> If you are concerned about security, change the custom path in config.

```bash
# Custom sub-store access path. Must start with /. Future subscription access must include this path.
# After setting a path, subscription sharing can be enabled without exposing the real path.
# sub-store-path: "/path"
sub-store-path: "/diypath"
```

The access path becomes `http://127.0.0.1:8299/diypath`.

![Step 10](./images/sub-store10.png)
![Step 11](./images/sub-store11.png)

## New sub-store Feature

sub-store backend version 2.19.97 and later supports GitHub proxying, used when sub-store syncs files and backs up or restores configuration. If token errors appear, check `githubproxy`.

> Backend must be >= 2.19.97. 1. It is only used to upload/download Gist and fetch GitHub avatars. 2. Enter the full URL, such as https://a.com. 3. The proxy must support both https://api.github.com/users/* and https://api.github.com/gists. Test by opening https://a.com/https://api.github.com/gists?per_page=1&page=1 and https://a.com/https://api.github.com/users/xream in a browser and confirming normal responses.

> Pay attention to security and privacy when using this method.
