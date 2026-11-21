# feed-subscriber

RSSフィードの新着記事をDiscordにMarkdown形式で投稿します。

## 設定と起動

```sh
cp secrets.yaml.example secrets.yaml
go run ./cmd/feed-subscriber
```

- `config.yaml`: Gitで管理するフィード、通知間隔、説明文の文字数制限、Webhookキー、秘密情報の参照
- `secrets.yaml`: Gitで管理しないWebhook URLとフィードURL用の秘密情報

`feeds[].webhook`には`secrets.yaml`と同じキーを指定します。フィードURL内の`${secret:<キー>}`は`secrets.yaml`の値に置換されます。状態ファイルには秘密情報を含むURLではなく、参照を含むURLが保存されます。既存の状態に秘密情報を含むURLがあれば、起動時にチェックポイントを保ったまま参照形式へ移行します。`secrets.yaml`と`state.json`はGitで追跡しません。

起動時に各フィードを確認しその後は設定間隔で巡回します。初回は記事を投稿せず最新記事を記録します。最後に確認した記事を`state.json`に記録し、再起動後の重複投稿を防ぎます。

## systemd

Linuxでsystemdサービスとして起動する場合は、バイナリと設定を配置して有効化します。

```sh
go build -o feed-subscriber ./cmd/feed-subscriber
sudo install -Dm755 feed-subscriber /usr/local/bin/feed-subscriber
sudo install -Dm600 config.yaml /etc/feed-subscriber/config.yaml
sudo install -Dm600 secrets.yaml /etc/feed-subscriber/secrets.yaml
sudo install -Dm644 systemd/feed-subscriber.service /etc/systemd/system/feed-subscriber.service
sudo systemctl daemon-reload
sudo systemctl enable --now feed-subscriber.service
```

サービスは動的ユーザーで動作し、設定と秘密情報をsystemdの資格情報機能で読み込み、状態は`/var/lib/feed-subscriber/`に保存します。

