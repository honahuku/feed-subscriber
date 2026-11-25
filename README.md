# feed-subscriber

RSSフィードの新着記事をDiscordにMarkdown形式で投稿します。

## 設定と起動

```sh
cp secrets.yaml.example secrets.yaml
go run ./cmd/feed-subscriber
```

- `config.yaml`: Gitで管理するフィード、通知間隔、説明文の文字数制限、Webhookキー、秘密情報の参照
- `secrets.yaml`: Gitで管理しないWebhook URLとフィードURL用の秘密情報

`error_webhook`と`feeds[].webhook`には`secrets.yaml`のキーを指定します。`error_webhook`はRSSソースの長期障害通知先です。フィードURL内の`${secret:<キー>}`は`secrets.yaml`の値に置換されます。状態ファイルには秘密情報を含むURLではなく、参照を含むURLが保存されます。既存の状態に秘密情報を含むURLがあれば、起動時にチェックポイントを保ったまま参照形式へ移行します。`secrets.yaml`、`state.json`、`state.json.failures.json`はGitで追跡しません。

起動時に各フィードを確認し、その後は設定間隔で巡回します。初回は記事を投稿せず最新記事を記録します。最後に確認した記事を`state.json`に記録し、再起動後の重複投稿を防ぎます。RSS取得・解析に失敗したURLは設定間隔から最大24時間までバックオフし、HTTP 2xxかつ解析成功で通常の間隔へ戻ります。障害が3日続くと`error_webhook`へ通知し、その後3日おきに最大3回追加通知します。障害状態は`state.json.failures.json`に保存します。

`feed-subscriber -version`で実行中バイナリのバージョンを確認できます。バージョンはUTCの年月日時分秒とミリ秒で、pre-commit時に自動更新されます。同じ時刻または時計が戻った場合は、直前のバージョンより1ミリ秒進めます。

## systemd

Linuxでsystemdサービスとして起動する場合は、repo内のunitを登録し、repo内の設定とGit管理外の秘密情報をsystemdの資格情報として読み込みます。repoはsystemd起動時に利用できる永続ファイルシステム上に置きます。

systemdの登録とdrop-inは`/etc/systemd/system/`に残りますが、アプリ設定と秘密情報のコピーは作りません。

```sh
go build -o feed-subscriber ./cmd/feed-subscriber
sudo install -Dm755 feed-subscriber /usr/local/bin/feed-subscriber
repo_dir="$(git rev-parse --show-toplevel)"
# unitのコピーを作らず、repo内のunitをsystemdへ登録する
sudo systemctl link "$repo_dir/systemd/feed-subscriber.service"
sudo systemctl edit feed-subscriber.service
```

`systemctl edit`で次のdrop-inを設定し、`/path/to/feed-subscriber`をrepoの絶対パスに置き換えます。空の`LoadCredential=`でunit本体の既定参照を消してから、repo内の設定と秘密情報を指定します。

```ini
[Service]
ConfigurationDirectory=
LoadCredential=
LoadCredential=config.yaml:/path/to/feed-subscriber/config.yaml
LoadCredential=secrets.yaml:/path/to/feed-subscriber/secrets.yaml
```

空の`ConfigurationDirectory=`はunit本体の`/etc/feed-subscriber/`作成を無効にします。`secrets.yaml`はGitで追跡せず、読み取り権限を所有者だけにします。すでにunitのコピーが登録されている場合は、そのコピーを取り除いてからrepoのunitをリンクします。unitの登録とdrop-inを設定した後、unitを再読み込みしてサービスを有効化します。

```sh
chmod 600 secrets.yaml
sudo systemctl daemon-reload
sudo systemctl enable --now feed-subscriber.service
```

サービスは動的ユーザーで動作し、systemdの資格情報機能を通じて設定と秘密情報を読み込みます。サービスプロセスにはrepoへのアクセス権を与えず、状態は`/var/lib/feed-subscriber/`に保存します。`main`をcheckoutしたrepoでmergeが完了すると、`post-merge` hookがこのunitの登録を確認し、バイナリを再ビルドして配置した後にサービスを再起動します。自動反映を有効にするには、repoのルートで一度`lefthook install`を実行してください。hook導入前のmergeにはhookがないため、自動反映の対象になりません。
