#!/bin/bash
# 把 .app 做成 DMG 安装包：打开后左边是 App，右边是“应用程序”的快捷方式，拖过去就装好了。
# 只能在 macOS 上运行（用系统自带的 hdiutil）。
#
#   scripts/package-dmg.sh build/bin/claude-code-sync.app dist/claude-code-sync-0.3.1-macos-arm64.dmg
set -euo pipefail

app="${1:?用法: package-dmg.sh <App 路径> <输出 dmg 路径>}"
out="${2:?用法: package-dmg.sh <App 路径> <输出 dmg 路径>}"
[ -d "$app" ] || { echo "找不到 $app" >&2; exit 1; }

stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT

# ditto 复制会保留 .app 里的符号链接、权限和签名
ditto "$app" "$stage/$(basename "$app")"
ln -s /Applications "$stage/Applications"

rm -f "$out"
hdiutil create -volname "Claude Code Sync" -srcfolder "$stage" -fs HFS+ -format UDZO -ov "$out"
hdiutil verify "$out"
