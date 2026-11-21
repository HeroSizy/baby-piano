#!/bin/bash
# 构建“宝宝钢琴.app”
# 需要：macOS 11 以上 + Xcode 命令行工具（没有的话先运行：xcode-select --install）
#
# 可选环境变量：
#   UNIVERSAL=1                 同时编译 Apple 芯片 + Intel（发布用）
#   SIGN_IDENTITY="Developer ID Application: 名字 (TEAMID)"
#                               用正式证书签名（默认 "-" = 本机自签名）
set -euo pipefail
cd "$(dirname "$0")"

APP="宝宝钢琴.app"
BIN="BabyPiano"
VERSION="${VERSION:-1.0.0}"
SIGN_IDENTITY="${SIGN_IDENTITY:--}"

if ! command -v swiftc >/dev/null 2>&1; then
  echo "❌ 找不到 swiftc。请先运行：xcode-select --install ，装好后再运行本脚本。"
  exit 1
fi

echo "▶ 编译中……"
rm -rf build "$APP"
mkdir -p build "$APP/Contents/MacOS" "$APP/Contents/Resources"

FLAGS=(-O -swift-version 5 -parse-as-library -framework Cocoa -framework AVFoundation -framework ApplicationServices)

if [[ "${UNIVERSAL:-0}" == "1" ]]; then
  swiftc main.swift "${FLAGS[@]}" -target arm64-apple-macos11  -o build/$BIN-arm64
  swiftc main.swift "${FLAGS[@]}" -target x86_64-apple-macos11 -o build/$BIN-x86_64
  lipo -create build/$BIN-arm64 build/$BIN-x86_64 -output "$APP/Contents/MacOS/$BIN"
else
  swiftc main.swift "${FLAGS[@]}" -o "$APP/Contents/MacOS/$BIN"
fi

cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key><string>$BIN</string>
  <key>CFBundleIdentifier</key><string>io.github.herosizy.babypiano</string>
  <key>CFBundleName</key><string>宝宝钢琴</string>
  <key>CFBundleDisplayName</key><string>宝宝钢琴</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>$VERSION</string>
  <key>CFBundleVersion</key><string>$VERSION</string>
  <key>LSMinimumSystemVersion</key><string>11.0</string>
  <key>LSApplicationCategoryType</key><string>public.app-category.kids-games</string>
  <key>NSHighResolutionCapable</key><true/>
  <key>NSPrincipalClass</key><string>NSApplication</string>
</dict>
</plist>
PLIST

if [[ "$SIGN_IDENTITY" == "-" ]]; then
  echo "▶ 本机自签名（ad-hoc）"
  codesign --force --deep --sign - "$APP"
else
  echo "▶ 使用证书签名：$SIGN_IDENTITY"
  codesign --force --deep --options runtime --timestamp --sign "$SIGN_IDENTITY" "$APP"
fi
codesign --verify --strict --verbose=2 "$APP"

echo "✅ 完成：$(pwd)/$APP"
