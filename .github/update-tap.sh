#!/usr/bin/env bash
set -euo pipefail

tag="$1"
version="${tag#v}"
url="https://github.com/${GITHUB_REPOSITORY}/archive/refs/tags/${tag}.tar.gz"
sha=$(curl -fsSL "$url" | shasum -a 256 | cut -d' ' -f1)

key=$(mktemp)
trap 'rm -f "$key"' EXIT
printf '%s\n' "$DEPLOY_KEY" > "$key"
chmod 600 "$key"
export GIT_SSH_COMMAND="ssh -i $key -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"

tap=$(mktemp -d)
git clone --depth 1 git@github.com:benteven/homebrew-tap.git "$tap"
mkdir -p "$tap/Formula"
sed -e "s|@URL@|$url|" -e "s|@SHA@|$sha|" .github/claude-deck.rb > "$tap/Formula/claude-deck.rb"

cd "$tap"
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
git add Formula/claude-deck.rb
git commit -m "claude-deck $version"
git push origin HEAD:main
