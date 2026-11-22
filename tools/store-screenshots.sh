#!/bin/sh
# Takes the frames the App Store screenshots are picked from, on a booted
# iOS simulator: TestStoreScreenshots poses the setup page, the keys and
# the keys with fireworks, and this captures the screen over and over
# meanwhile, status bar and fireworks in flight included.
#
#   tools/store-screenshots.sh <simulator udid> <frames directory>
#
# Put the simulator in dark mode and English first, with a clean status bar:
#
#   xcrun simctl ui <udid> appearance dark
#   xcrun simctl status_bar <udid> override --time 9:41 --batteryState charged --batteryLevel 100
#
# Then pick one frame of each scene from the directory. They are the
# device's full resolution: 1320x2868 on an iPhone 17 Pro Max (6.9"),
# 2064x2752 on an iPad Pro 13-inch.
set -eu
udid=$1
out=$2
mkdir -p "$out"
cd "$(dirname "$0")/.."

go tool nsgo test -target ios_simulator -device "$udid" -env STORE_SCREENSHOTS=1 -run TestStoreScreenshots . >"$out/test.log" 2>&1 &
test=$!
i=0
while kill -0 "$test" 2>/dev/null; do
	i=$((i + 1))
	xcrun simctl io "$udid" screenshot "$out/frame-$(printf %03d "$i").png" >/dev/null 2>&1 || true
done
wait "$test" || { tail -20 "$out/test.log"; exit 1; }
echo "$i frames in $out"
