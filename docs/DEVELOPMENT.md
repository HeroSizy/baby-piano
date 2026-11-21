# 开发、编译与发布

宝宝钢琴用 Go 和 [NSGo](https://github.com/nsgo-dev/nsgo) 写成，一份代码支持 Mac、Windows、iPhone、iPad 和 Android。旧的 Swift 版本留在 [`legacy/macos-swift`](../legacy/macos-swift)。

## 自己编译

需要 Go（版本见 `.mise.toml`）。

```bash
go tool nsgo dev                                 # 在本机运行
go tool nsgo test ./...                          # 测试
go tool nsgo build -target macos_arm64           # Mac
go tool nsgo build -target windows_x64           # Windows（在 Mac 上也能编译）
go tool nsgo build -target android_arm64         # Android
go tool nsgo build -target ios                   # iPhone / iPad（需要 Xcode）
```

采样从 Salamander Grand Piano 的 FLAC 重新生成：`tools/make-samples.sh <salamander 目录>`（需要 ffmpeg）。

## 发布与签名

GitHub Actions 自动发版：把 `nsgo.yaml` 里的 `version` 改成新版本号（比如 `1.0.1`），合并进 `main`，就会编译所有平台、打上 `v1.0.1` 标签并发布 Release，附上 Mac、Windows、Android 安装包。版本号已经有标签时只编译、不发版；手动推送 `v*` 标签也照样发版。

Mac 的签名和公证用仓库 Secrets：

| Secret | 内容 |
|---|---|
| `MACOS_CERT_P12` | “Developer ID Application” 证书导出的 .p12，做 base64 |
| `MACOS_CERT_PASSWORD` | 导出 .p12 时设的密码 |
| `APPLE_API_KEY_P8` | App Store Connect API 密钥的 .p8 文件内容（公证用，首选） |
| `APPLE_API_KEY_ID` | 该密钥的 Key ID |
| `APPLE_API_ISSUER` | 该密钥的 Issuer ID |
| `APPLE_ID` | Apple 开发者账号邮箱（公证的另一种方式） |
| `APPLE_APP_PASSWORD` | 在 account.apple.com 生成的“App 专用密码” |
| `APPLE_TEAM_ID` | 10 位 Team ID（可选，默认 HK2CBGL2Q2，仅 Apple ID 方式用） |

只配前两个 = 正式签名但不公证；再配三个 `APPLE_API_*`（或者 `APPLE_ID` 和 `APPLE_APP_PASSWORD`）= 签名 + 公证。两种都配时用 API 密钥。

采样的来源和许可见 [README 的致谢](../README.md#致谢)。
