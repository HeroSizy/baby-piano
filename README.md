# 🎹 宝宝钢琴 BabyPiano

给小宝宝用的钢琴：宝宝怎么乱按，都只会弹出好听的钢琴声。电脑上 **不会退出、切走程序或触发系统快捷键**；手机和平板上是一排排大大的彩色琴键。

支持 **Mac、Windows、iPhone、iPad、Android**，用 Go 和 [NSGo](https://github.com/nsgo-dev/nsgo) 写成，一份代码。旧的 Swift 版本留在 [`legacy/macos-swift`](legacy/macos-swift)。

## 它能做什么

- **乱按也好听**：所有音都在 C 大调五声音阶里（C D E G A），从 C1 到 C8 共 36 个音，任意组合都和谐。
- **整块键盘都能弹**：字母、数字、符号、F1–F12、方向键、小键盘、Ctrl/Alt/Cmd/Shift……每个键都有自己的音。键盘每往上一排高一个八度，同一排从左到右越来越高；同一个键永远是同一个音。
- **真钢琴音色**：Salamander Grand Piano 采样（雅马哈 C5 三角钢琴）。
- **保护耳朵**：高音自动更轻；一巴掌按下十几个键只会响几个；弹琴时电脑系统音量最高 50%，中途连上蓝牙音箱/耳机也会自动压回去。
- **默认黑屏**：宝宝只听声音；想要的话可以勾选“彩色圆圈”。
- 玩的时候屏幕不熄灭、电脑不休眠。

## 电脑（Mac、Windows）

1. 打开 **宝宝钢琴**，先用滑块调好音量，点 **试听**（开始后键盘上的音量键会被锁住）。
2. 点 **开始弹琴（全屏锁定键盘）**，把键盘交给宝宝。
3. **家长退出**（两种都行）：
   - 用鼠标在屏幕 **右上角** 按住不放 **3 秒**（勾选了“锁住鼠标”时不可用）；
   - 或在键盘上连续输入 **`quitpiano`**（字母之间停顿不超过 3 秒）。

笔记本建议勾选 **“同时锁住鼠标和触控板”**：光标不动、点击无效，退出只能输入 `quitpiano`。

### Mac

- 下载 `macos-arm64.dmg`（Apple 芯片）或 `macos-x64.dmg`（Intel），把「宝宝钢琴」拖进「应用程序」。
- 第一次使用要授予 **辅助功能** 权限（系统设置 › 隐私与安全性 › 辅助功能 › 打开“宝宝钢琴”）。拦截 Cmd+Q、Cmd+Tab 这类系统快捷键，macOS 只允许有这个权限的程序做。本程序不联网，也不记录任何按键。
- 弹琴时 Dock、菜单栏、Cmd+Tab、调度中心、触控板手势、媒体键都被拦下；多个显示器时其他屏幕盖上黑色窗口。
- **拦不住的**：电源键 / Touch ID 键、合上屏幕、旧款 Touch Bar 上的按钮。其他程序开启了“安全键盘输入”（终端、密码框）时键盘锁不住，程序会提示。
- 万一程序卡住：按 **Cmd+Option+Esc** 强制退出。
- 如果 Release 说明写的是“本机自签名”，第一次打开会提示“无法验证开发者”：到 系统设置 › 隐私与安全性 最下面点“仍要打开”。

### Windows

- 下载 `windows-x64-setup.exe`（大多数电脑）或 `windows-arm64-setup.exe`（ARM 电脑）安装。不需要管理员权限。
- 弹琴时 Win 键、Alt+Tab、Alt+F4、媒体键都被拦下；连按 5 次 Shift 之类的辅助功能快捷键和 **Win+L 锁屏** 会暂时关掉，退出后（即使程序意外退出，下次打开时）自动恢复。
- **拦不住的**：Ctrl+Alt+Del（任何程序都拦不住）、电源键、合上屏幕。

## 手机和平板（iPhone、iPad、Android）

大大的彩色琴键，宝宝每根手指都能弹（可以多指同时弹）。家长在右上角按住 3 秒回到设置页。

- **iPhone / iPad**：想让宝宝出不去，打开“引导式访问”：设置 › 辅助功能 › 引导式访问，弹琴时连按三下侧边按钮。
- **Android**：下载 `.apk` 安装。想让宝宝出不去，打开“应用固定”：设置 › 安全 › 应用固定，然后在最近任务里固定“宝宝钢琴”。

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

## 发布与签名（维护者）

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

## 致谢

钢琴采样：[Salamander Grand Piano V3](https://archive.org/details/SalamanderGrandPianoV3) by Alexander Holm，[CC BY 3.0](https://creativecommons.org/licenses/by/3.0/)。`sound/notes` 里的文件经过裁剪、淡出和音量统一。
