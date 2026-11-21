# feed-subscriber

A small RSS-to-Discord webhook subscriber.

## Setup

Copy `config.yaml.example` to `config.yaml` and `secrets.yaml.example` to
`secrets.yaml`.

`config.yaml` contains feed URLs, polling intervals, and a webhook key for each
destination. `secrets.yaml` contains only key-to-webhook-URL pairs. The key in
`feeds[].webhook` must match a key in `secrets.yaml`. Keep `secrets.yaml` private;
both local files and the generated `state.json` are ignored by Git.

```yaml
# config.yaml
feeds:
  - name: engineering
    webhook: announcements
    interval: 30m
    description_limit: 10
    urls:
      - https://example.com/feed.xml
```

```yaml
# secrets.yaml
announcements: https://discord.com/api/webhooks/replace-me
```

Start with the latest stable Go release:

```sh
go run ./cmd/feed-subscriber
```

The process checks each feed immediately, then repeats at its configured
interval. New items are posted as Discord Markdown messages. The first check
records the newest item without posting old history. `state.json` stores the
last delivered item per feed to avoid reposting after restarts.
