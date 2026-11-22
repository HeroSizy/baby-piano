# Developing, building and releasing

Mash Piano is written in Go with [NSGo](https://github.com/nsgo-dev/nsgo): one codebase for Mac, Windows, iPhone, iPad and Android. The earlier Swift version is kept in [`legacy/macos-swift`](../legacy/macos-swift).

## Building

You need Go (the version is in `.mise.toml`).

```bash
go tool nsgo dev                                 # run on this machine
go tool nsgo test ./...                          # tests
go tool nsgo build -target macos_arm64           # Mac
go tool nsgo build -target windows_x64           # Windows (builds on a Mac too)
go tool nsgo build -target android_arm64         # Android
go tool nsgo build -target ios                   # iPhone / iPad (needs Xcode)
```

The samples are regenerated from Salamander Grand Piano's FLAC files with `tools/make-samples.sh <salamander directory>` (needs ffmpeg).

The App Store screenshots in `docs/store/screenshots` come from `tools/store-screenshots.sh <simulator udid> <frames directory>` on an iPhone 17 Pro Max (6.9", 1320×2868) and an iPad Pro 13-inch (2064×2752) simulator in dark mode and English. It runs `TestStoreScreenshots`, which shows the setup page, the keys, and the keys being played with fireworks, and captures the screen meanwhile; pick one frame of each scene and save it without an alpha channel (`ffmpeg -i frame.png -pix_fmt rgb24 out.png`), as App Store Connect wants.

## Releasing and signing

Every push to `main` is a release; there are no pull requests. GitHub Actions builds every platform, picks the next version and publishes a GitHub Release with the Mac, Windows and Android packages and the changes since the last one. The version is `nsgo.yaml`'s, or when that already has a tag, the next patch after the highest tag of its major.minor (1.0.3, 1.0.4, 1.0.5 …); to start 1.1 or 2.0, set `version` to `1.1.0` or `2.0.0`. The iPhone/iPad build goes up to App Store Connect with build number = commit count + 100, so it rises by one per commit, and TestFlight gives it to the internal group Family by itself. Submitting a version for App Store review stays a manual step in App Store Connect. Pushing a `v*` tag by hand releases that version.

Mac signing and notarization use repository secrets:

| Secret | Value |
|---|---|
| `MACOS_CERT_P12` | The "Developer ID Application" certificate exported as .p12, base64-encoded |
| `MACOS_CERT_PASSWORD` | The password set when exporting the .p12 |
| `APPLE_API_KEY_P8` | The contents of an App Store Connect API key's .p8 file (for notarization, preferred) |
| `APPLE_API_KEY_ID` | That key's Key ID |
| `APPLE_API_ISSUER` | That key's Issuer ID |
| `APPLE_ID` | The Apple developer account's email (the other way to notarize) |
| `APPLE_APP_PASSWORD` | An app-specific password created at account.apple.com |
| `APPLE_TEAM_ID` | The 10-character Team ID (optional, defaults to HK2CBGL2Q2; Apple ID route only) |

| `IOS_DIST_CERT_P12` | The "Apple Distribution" certificate exported as .p12, base64-encoded (App Store builds) |
| `IOS_DIST_CERT_PASSWORD` | The password set when exporting that .p12 |
| `IOS_PROFILE` | The App Store provisioning profile for dev.herosizy.babypiano, base64-encoded |

With only the first two, the app is signed but not notarized. Adding the three `APPLE_API_*` secrets (or `APPLE_ID` and `APPLE_APP_PASSWORD`) signs and notarizes it. When both kinds are set, the API key is used.

With the three `IOS_*` secrets and the `APPLE_API_*` key, every release also signs the iPhone/iPad package for the App Store and uploads it to App Store Connect. Submitting the build for review is still done by hand in App Store Connect.

The samples' source and license are in the [README's credits](../README.md#credits).
