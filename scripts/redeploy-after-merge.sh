#!/bin/bash
#
# mainへのmerge後、登録済みサービスを再ビルドして再起動する。
set -euo pipefail

repo_dir="$(git rev-parse --show-toplevel)"
expected_unit="$repo_dir/systemd/feed-subscriber.service"

if [[ "$(git branch --show-current)" != "main" ]]; then
  exit 0
fi

unit_path="/etc/systemd/system/feed-subscriber.service"
if [[ "$(readlink -f "$unit_path")" != "$expected_unit" ]]; then
  exit 0
fi

binary="$(mktemp)"
trap 'rm -f -- "$binary"' EXIT

go build -o "$binary" ./cmd/feed-subscriber
sudo install -Dm755 "$binary" /usr/local/bin/feed-subscriber
sudo systemctl restart feed-subscriber.service
sudo systemctl is-active --quiet feed-subscriber.service
