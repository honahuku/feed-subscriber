#!/bin/bash
#
# mainでrepoのunitが登録済みの場合だけ再配置することを検証する。
set -euo pipefail

repo_dir="$(git rev-parse --show-toplevel)"
hook_script="$repo_dir/scripts/redeploy-after-merge.sh"
test_dir="$(mktemp -d)"
call_log="$test_dir/calls"
fake_bin="$test_dir/bin"
real_git="$(command -v git)"

mkdir -p "$fake_bin"

cat >"$fake_bin/git" <<'EOF'
#!/bin/bash
if [[ "${1:-}" == "branch" && "${2:-}" == "--show-current" ]]; then
  printf '%s\n' "${TEST_BRANCH:-feature}"
else
  exec "$REAL_GIT" "$@"
fi
EOF

cat >"$fake_bin/readlink" <<'EOF'
#!/bin/bash
if [[ "${TEST_UNIT_LINKED:-}" == "yes" ]]; then
  printf '%s/systemd/feed-subscriber.service\n' "$TEST_REPO"
else
  printf '%s\n' /not-the-repository-unit
fi
EOF

cat >"$fake_bin/go" <<'EOF'
#!/bin/bash
while [[ "$#" -gt 0 ]]; do
  if [[ "$1" == "-o" ]]; then
    shift
    : >"$1"
    exit 0
  fi
  shift
done
exit 1
EOF

cat >"$fake_bin/sudo" <<'EOF'
#!/bin/bash
printf '%s\n' "$*" >>"$CALL_LOG"
EOF

chmod +x "$fake_bin/git" "$fake_bin/readlink" "$fake_bin/go" "$fake_bin/sudo"
trap 'rm -rf -- "$test_dir"' EXIT

run_hook() {
  : >"$call_log"
  TEST_BRANCH="$1" \
    TEST_UNIT_LINKED="$2" \
    TEST_REPO="$repo_dir" \
    REAL_GIT="$real_git" \
    CALL_LOG="$call_log" \
    PATH="$fake_bin:$PATH" \
    "$hook_script"
}

run_hook feature yes
if [[ -s "$call_log" ]]; then
  echo "feature branch unexpectedly ran deployment commands" >&2
  exit 1
fi

run_hook main no
if [[ -s "$call_log" ]]; then
  echo "unregistered unit unexpectedly ran deployment commands" >&2
  exit 1
fi

run_hook main yes
grep -Fq 'install -Dm755 ' "$call_log"
grep -Fq 'systemctl restart feed-subscriber.service' "$call_log"
grep -Fq 'systemctl is-active --quiet feed-subscriber.service' "$call_log"
