# Gist Save Method

## Deployment

- Create a Gist.
- Configure the Gist ID in `config.yaml`.
- Configure the Gist token in `config.yaml`.

## Worker Reverse Proxy For GitHub API

- Deploy [worker](./cloudflare/worker.js) to Cloudflare Workers.
- In `Variables and Secrets`, set `GITHUB_USER` to your GitHub username.
- In `Variables and Secrets`, set `GITHUB_ID` to your Gist ID.
- In `Variables and Secrets`, set `AUTH_TOKEN` as the access token.
- Set `github-api-mirror` to your Worker URL.

```yaml
github-api-mirror: "https://your-worker-url/github"
```

## Get Subscriptions

> If a Worker is configured, change `key` to the corresponding filename.
> Subscription format: `https://your-worker-url/gist?key=all.yaml&token=AUTH_TOKEN`

- YAML subscription:

```text
https://gist.githubusercontent.com/YOUR_GITHUB_USERNAME/YOUR_GIST_ID/raw/all.yaml
```

- Base64 subscription:

```text
https://gist.githubusercontent.com/YOUR_GITHUB_USERNAME/YOUR_GIST_ID/raw/base64.txt
```

- Rule-based `mihomo.yaml` file:

```text
https://gist.githubusercontent.com/YOUR_GITHUB_USERNAME/YOUR_GIST_ID/raw/mihomo.yaml
```
