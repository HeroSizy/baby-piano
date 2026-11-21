// 宝宝钢琴 BabyPiano —— macOS 原生 (AppKit) 单文件应用
//
// 功能：
//  - 全局拦截所有键盘事件（含 Cmd+Q / Cmd+Tab / Cmd+Space / 功能键 / 媒体键 / 触控板手势），
//    一个都不交给系统，宝宝怎么按都不会退出或切走。
//  - 每个键对应一个 C 大调五声音阶里的钢琴音，乱按也好听；同一个键永远是同一个音。
//  - 全屏柔和彩色圆圈动画。
//  - 家长退出：用鼠标在屏幕右上角长按 3 秒。
//
// 构建：见同目录 build.sh

import Cocoa
import AVFoundation
import ApplicationServices

// MARK: - 设置

enum Settings {
    static var volume: Float {
        get {
            let v = UserDefaults.standard.object(forKey: "volume") as? Float
            return v ?? 0.45
        }
        set { UserDefaults.standard.set(newValue, forKey: "volume") }
    }
    /// 宝宝一般看不到屏幕，默认不显示动画（纯黑屏，省电也不分散注意）
    static var showVisuals: Bool {
        get { UserDefaults.standard.bool(forKey: "showVisuals") }
        set { UserDefaults.standard.set(newValue, forKey: "showVisuals") }
    }
}

// MARK: - 声音

final class PianoSound: NSObject {
    private let engine = AVAudioEngine()
    private let sampler = AVAudioUnitSampler()
    private let limiter: AVAudioUnitEffect
    private var generation = [UInt8: Int]()
    private var recentHits: [CFTimeInterval] = []

    /// C 大调五声音阶 C3 ~ C6（任意组合都和谐）
    let scale: [UInt8]
    private(set) var usingPianoBank = false

    override init() {
        var notes: [UInt8] = []
        let pentatonic = [0, 2, 4, 7, 9]
        for octave in 0..<3 {
            for p in pentatonic { notes.append(UInt8(48 + octave * 12 + p)) }
        }
        notes.append(84)
        scale = notes

        let desc = AudioComponentDescription(
            componentType: kAudioUnitType_Effect,
            componentSubType: kAudioUnitSubType_PeakLimiter,
            componentManufacturer: kAudioUnitManufacturer_Apple,
            componentFlags: 0,
            componentFlagsMask: 0)
        limiter = AVAudioUnitEffect(audioComponentDescription: desc)
        super.init()

        engine.attach(sampler)
        engine.attach(limiter)
        engine.connect(sampler, to: limiter, format: nil)
        engine.connect(limiter, to: engine.mainMixerNode, format: nil)
        engine.mainMixerNode.outputVolume = Settings.volume

        loadInstrument()
        startEngine()

        NotificationCenter.default.addObserver(
            self, selector: #selector(engineConfigChanged),
            name: .AVAudioEngineConfigurationChange, object: engine)
    }

    private func loadInstrument() {
        // 1) 家长自备音色：~/Music/BabyPiano.sf2（可选）
        let custom = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent("Music/BabyPiano.sf2")
        // 2) macOS 自带的 General MIDI 音色库（0 号音色 = 大钢琴）
        let builtin = URL(fileURLWithPath:
            "/System/Library/Components/CoreAudio.component/Contents/Resources/gs_instruments.dls")
        for url in [custom, builtin] where FileManager.default.fileExists(atPath: url.path) {
            do {
                try sampler.loadSoundBankInstrument(
                    at: url, program: 0,
                    bankMSB: UInt8(kAUSampleBankMSBDefault),
                    bankLSB: UInt8(kAUSampleBankLSBDefault))
                usingPianoBank = true
                NSLog("BabyPiano: loaded instrument \(url.path)")
                return
            } catch {
                NSLog("BabyPiano: failed to load \(url.path): \(error)")
            }
        }
        NSLog("BabyPiano: using default sampler tone")
    }

    private func startEngine() {
        do { try engine.start() } catch { NSLog("BabyPiano: engine start failed: \(error)") }
    }

    @objc private func engineConfigChanged(_ note: Notification) {
        // 插拔耳机/切换输出设备后引擎会停，重新启动
        DispatchQueue.main.async { [weak self] in
            guard let self = self else { return }
            if !self.engine.isRunning { self.startEngine() }
        }
    }

    func setVolume(_ v: Float) {
        engine.mainMixerNode.outputVolume = v
        Settings.volume = v
    }

    /// 播放音阶中第 index 个音。返回 false 表示被限流（宝宝一巴掌按下十几个键时）。
    @discardableResult
    func play(index: Int) -> Bool {
        let now = CACurrentMediaTime()
        recentHits = recentHits.filter { now - $0 < 0.3 }
        if recentHits.count >= 8 { return false }
        recentHits.append(now)

        if !engine.isRunning { startEngine() }

        let note = scale[max(0, min(scale.count - 1, index))]
        let velocity = UInt8.random(in: 58...72)   // 中等力度，不刺耳
        sampler.stopNote(note, onChannel: 0)
        sampler.startNote(note, withVelocity: velocity, onChannel: 0)

        let g = (generation[note] ?? 0) + 1
        generation[note] = g
        DispatchQueue.main.asyncAfter(deadline: .now() + 2.5) { [weak self] in
            guard let self = self, self.generation[note] == g else { return }
            self.sampler.stopNote(note, onChannel: 0)
        }
        return true
    }
}

// MARK: - 键位 -> 音高（左低右高，上排略高）

enum KeyMap {
    // ANSI 键盘的 virtual keycode，按物理行排列（左 -> 右）
    static let rows: [[Int]] = [
        [50, 18, 19, 20, 21, 23, 22, 26, 28, 25, 29, 27, 24, 51],   // ` 1 2 ... = Delete
        [48, 12, 13, 14, 15, 17, 16, 32, 34, 31, 35, 33, 30, 42],   // Tab Q W ... ] \
        [57, 0, 1, 2, 3, 5, 4, 38, 40, 37, 41, 39, 36],             // Caps A S ... ' Return
        [56, 6, 7, 8, 9, 11, 45, 46, 43, 47, 44, 60],               // Shift Z X ... / RShift
    ]

    static let table: [Int: Int] = {
        var t = [Int: Int]()
        for (r, row) in rows.enumerated() {
            for (c, code) in row.enumerated() {
                let base = Int((Double(c) / 13.0 * 12.0).rounded())
                t[code] = min(15, max(0, base + (3 - r)))
            }
        }
        return t
    }()

    static func noteIndex(forKeyCode code: Int, scaleCount: Int) -> Int {
        if let i = table[code] { return min(i, scaleCount - 1) }
        // 其他键（功能键、方向键、空格、媒体键……）：固定但分散的映射
        return (code * 7) % scaleCount
    }
}

// MARK: - 全局键盘锁（CGEventTap）

/// 事件回调是 C 函数指针，不能捕获上下文，所以用单例。
final class KeyboardLock {
    static let shared = KeyboardLock()
    fileprivate(set) var tap: CFMachPort?
    private var source: CFRunLoopSource?
    private var pressedModifiers = Set<Int>()

    /// 收到“按下”的键时回调（在主线程）
    var onKeyDown: ((Int) -> Void)?

    var isActive: Bool { tap != nil }

    func start() -> Bool {
        if tap != nil { return true }

        var mask: CGEventMask = 0
        // 10 keyDown, 11 keyUp, 12 flagsChanged, 14 系统定义事件（媒体键/亮度/电源键等）
        // 18/19/20/29/30/31/32 触控板手势（防止三指/四指滑动切桌面、调度中心）
        for raw in [10, 11, 12, 14, 18, 19, 20, 29, 30, 31, 32] as [UInt64] {
            mask |= (1 << raw)
        }

        guard let t = CGEvent.tapCreate(
            tap: .cghidEventTap,
            place: .headInsertEventTap,
            options: .defaultTap,
            eventsOfInterest: mask,
            callback: keyboardLockCallback,
            userInfo: nil) else {
            return false
        }
        tap = t
        source = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, t, 0)
        CFRunLoopAddSource(CFRunLoopGetMain(), source, .commonModes)
        CGEvent.tapEnable(tap: t, enable: true)
        return true
    }

    func stop() {
        if let t = tap { CGEvent.tapEnable(tap: t, enable: false) }
        if let s = source { CFRunLoopRemoveSource(CFRunLoopGetMain(), s, .commonModes) }
        tap = nil
        source = nil
    }

    fileprivate func handle(type: CGEventType, event: CGEvent) {
        switch type.rawValue {
        case 10: // keyDown
            if event.getIntegerValueField(.keyboardEventAutorepeat) != 0 { return }  // 按住不放不重复响
            let code = Int(event.getIntegerValueField(.keyboardEventKeycode))
            fire(code)

        case 12: // flagsChanged：Shift/Ctrl/Option/Cmd/Fn/CapsLock，按下和松开都会来一次
            let code = Int(event.getIntegerValueField(.keyboardEventKeycode))
            if pressedModifiers.contains(code) {
                pressedModifiers.remove(code)
            } else {
                pressedModifiers.insert(code)
                fire(code)
            }

        case 14: // 系统定义事件：媒体键、亮度、音量等
            guard let ns = NSEvent(cgEvent: event), ns.subtype.rawValue == 8 else { return }
            let data1 = ns.data1
            let keyCode = (data1 & 0xFFFF0000) >> 16
            let keyFlags = data1 & 0x0000FFFF
            let isDown = ((keyFlags & 0xFF00) >> 8) == 0x0A
            let isRepeat = (keyFlags & 0x1) == 1
            if isDown && !isRepeat { fire(200 + keyCode) }

        default:
            break // keyUp、手势：直接吞掉
        }
    }

    private func fire(_ code: Int) {
        DispatchQueue.main.async {
            KeyboardLock.shared.onKeyDown?(code)
        }
    }
}

/// 吞掉所有关注的事件（返回 nil），系统和其他应用都收不到。
private func keyboardLockCallback(
    proxy: CGEventTapProxy,
    type: CGEventType,
    event: CGEvent,
    refcon: UnsafeMutableRawPointer?
) -> Unmanaged<CGEvent>? {
    if type == .tapDisabledByTimeout || type == .tapDisabledByUserInput {
        // 系统可能因超时临时关闭 tap，立刻重新打开
        if let t = KeyboardLock.shared.tap { CGEvent.tapEnable(tap: t, enable: true) }
        return Unmanaged.passUnretained(event)
    }
    KeyboardLock.shared.handle(type: type, event: event)
    return nil
}

// MARK: - 全屏游戏画面

final class KioskWindow: NSWindow {
    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { true }
}

final class PlayView: NSView {
    var onExit: (() -> Void)?
    var onClickNote: ((CGPoint) -> Void)?

    private let exitSize: CGFloat = 90
    private let exitHold: TimeInterval = 3.0
    private var exitTimer: Timer?
    private let ringLayer = CAShapeLayer()
    private let ringTrack = CAShapeLayer()
    private let hintLabel = NSTextField(labelWithString: "家长：鼠标在右上角长按 3 秒退出")

    override init(frame: NSRect) {
        super.init(frame: frame)
        wantsLayer = true
        layer?.backgroundColor = Settings.showVisuals
            ? NSColor(calibratedRed: 0.07, green: 0.08, blue: 0.17, alpha: 1).cgColor
            : NSColor.black.cgColor

        for l in [ringTrack, ringLayer] {
            l.fillColor = NSColor.clear.cgColor
            l.lineWidth = 4
            l.lineCap = .round
            layer?.addSublayer(l)
        }
        ringTrack.strokeColor = NSColor(white: 1, alpha: 0.12).cgColor
        ringLayer.strokeColor = NSColor(white: 1, alpha: 0.85).cgColor
        ringLayer.strokeEnd = 0
        ringLayer.zPosition = 1000
        ringTrack.zPosition = 999

        hintLabel.textColor = NSColor(white: 1, alpha: 0.25)
        hintLabel.font = NSFont.systemFont(ofSize: 11)
        hintLabel.alignment = .right
        addSubview(hintLabel)
    }

    required init?(coder: NSCoder) { fatalError() }

    override var acceptsFirstResponder: Bool { true }
    override func acceptsFirstMouse(for event: NSEvent?) -> Bool { true }

    private var exitRect: NSRect {
        NSRect(x: bounds.maxX - exitSize, y: bounds.maxY - exitSize, width: exitSize, height: exitSize)
    }

    override func layout() {
        super.layout()
        let r = exitRect.insetBy(dx: 25, dy: 25)
        let path = CGPath(ellipseIn: r, transform: nil)
        ringTrack.path = path
        ringLayer.path = path
        hintLabel.frame = NSRect(x: bounds.maxX - 260, y: bounds.maxY - exitSize - 18, width: 245, height: 16)
    }

    // MARK: 彩色圆圈

    func burst(noteIndex: Int, noteCount: Int, at point: CGPoint? = nil) {
        guard Settings.showVisuals, let root = layer else { return }
        if (root.sublayers?.count ?? 0) > 70 { return }

        let t = CGFloat(noteIndex) / CGFloat(max(noteCount - 1, 1))
        let color = NSColor(calibratedHue: t * 0.85, saturation: 0.55, brightness: 1.0, alpha: 1)
        let size = CGFloat.random(in: 150...280)
        let x = point?.x ?? (bounds.width * (0.08 + 0.84 * t) + CGFloat.random(in: -50...50))
        let y = point?.y ?? CGFloat.random(in: bounds.height * 0.15...bounds.height * 0.85)

        let circle = CAShapeLayer()
        circle.path = CGPath(ellipseIn: CGRect(x: -size / 2, y: -size / 2, width: size, height: size), transform: nil)
        circle.position = CGPoint(x: x, y: y)
        circle.fillColor = color.cgColor
        circle.opacity = 0

        let scale = CABasicAnimation(keyPath: "transform.scale")
        scale.fromValue = 0.25
        scale.toValue = 1.35

        let fade = CAKeyframeAnimation(keyPath: "opacity")
        fade.values = [0.0, 0.85, 0.0]
        fade.keyTimes = [0, 0.15, 1]

        let group = CAAnimationGroup()
        group.animations = [scale, fade]
        group.duration = 1.8
        group.timingFunction = CAMediaTimingFunction(name: .easeOut)

        root.addSublayer(circle)
        CATransaction.begin()
        CATransaction.setCompletionBlock { circle.removeFromSuperlayer() }
        circle.add(group, forKey: "burst")
        CATransaction.commit()
    }

    // MARK: 鼠标：右上角长按退出，其他地方点一下也会响

    override func mouseDown(with event: NSEvent) {
        let p = convert(event.locationInWindow, from: nil)
        if exitRect.contains(p) {
            startExitHold()
        } else {
            onClickNote?(p)
        }
    }

    override func mouseDragged(with event: NSEvent) {
        let p = convert(event.locationInWindow, from: nil)
        if exitTimer != nil && !exitRect.contains(p) { cancelExitHold() }
    }

    override func mouseUp(with event: NSEvent) {
        cancelExitHold()
    }

    private func startExitHold() {
        cancelExitHold()
        let anim = CABasicAnimation(keyPath: "strokeEnd")
        anim.fromValue = 0
        anim.toValue = 1
        anim.duration = exitHold
        ringLayer.strokeEnd = 1
        ringLayer.add(anim, forKey: "progress")
        exitTimer = Timer.scheduledTimer(timeInterval: exitHold, target: self,
                                         selector: #selector(exitTimerFired),
                                         userInfo: nil, repeats: false)
    }

    private func cancelExitHold() {
        exitTimer?.invalidate()
        exitTimer = nil
        ringLayer.removeAnimation(forKey: "progress")
        ringLayer.strokeEnd = 0
    }

    @objc private func exitTimerFired() {
        exitTimer = nil
        onExit?()
    }
}

// MARK: - 启动设置窗口

@MainActor
final class SetupController: NSObject {
    let window: NSWindow
    private let statusLabel = NSTextField(labelWithString: "")
    private let startButton = NSButton(title: "开始弹琴（全屏锁定键盘）", target: nil, action: nil)
    private let openSettingsButton = NSButton(title: "打开“辅助功能”设置", target: nil, action: nil)
    private var pollTimer: Timer?
    private let sound: PianoSound
    var onStart: (() -> Void)?

    init(sound: PianoSound) {
        self.sound = sound
        window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 560, height: 420),
                          styleMask: [.titled, .closable, .miniaturizable],
                          backing: .buffered, defer: false)
        super.init()
        window.title = "宝宝钢琴"
        window.isReleasedWhenClosed = false
        buildUI()
        window.center()
    }

    private func label(_ s: String, size: CGFloat, bold: Bool = false, color: NSColor = .labelColor) -> NSTextField {
        let l = NSTextField(wrappingLabelWithString: s)
        l.font = bold ? NSFont.boldSystemFont(ofSize: size) : NSFont.systemFont(ofSize: size)
        l.textColor = color
        l.preferredMaxLayoutWidth = 480
        return l
    }

    private func buildUI() {
        let title = label("🎹 宝宝钢琴", size: 26, bold: true)
        let desc = label("开始后会全屏，并接管整块键盘：宝宝随便按都只会弹出好听的钢琴声，不会退出、切换程序或触发系统快捷键。",
                         size: 13, color: .secondaryLabelColor)
        let exitTip = label("家长退出方法：用鼠标在屏幕【右上角】按住不放 3 秒。",
                            size: 13, bold: true)

        statusLabel.font = NSFont.systemFont(ofSize: 13)
        statusLabel.preferredMaxLayoutWidth = 480
        statusLabel.lineBreakMode = .byWordWrapping
        statusLabel.maximumNumberOfLines = 0

        openSettingsButton.target = self
        openSettingsButton.action = #selector(openAccessibilitySettings)
        openSettingsButton.bezelStyle = .rounded

        let volLabel = label("音量（开始后键盘上的音量键会被锁住，请先在这里调好）", size: 12, color: .secondaryLabelColor)
        let slider = NSSlider(value: Double(Settings.volume), minValue: 0.05, maxValue: 0.9,
                              target: self, action: #selector(volumeChanged(_:)))
        slider.widthAnchor.constraint(equalToConstant: 260).isActive = true
        let testButton = NSButton(title: "试听", target: self, action: #selector(testSound))
        testButton.bezelStyle = .rounded
        let volRow = NSStackView(views: [slider, testButton])
        volRow.orientation = .horizontal

        let visualsBox = NSButton(checkboxWithTitle: "按键时显示彩色动画（宝宝看不到屏幕的话不用开）",
                                  target: self, action: #selector(visualsToggled(_:)))
        visualsBox.state = Settings.showVisuals ? .on : .off

        startButton.target = self
        startButton.action = #selector(startPressed)
        startButton.bezelStyle = .rounded
        startButton.controlSize = .large
        startButton.keyEquivalent = "\r"

        let stack = NSStackView(views: [title, desc, exitTip, statusLabel, openSettingsButton, volLabel, volRow, visualsBox, startButton])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 12
        stack.edgeInsets = NSEdgeInsets(top: 24, left: 32, bottom: 24, right: 32)
        stack.setCustomSpacing(20, after: visualsBox)
        stack.translatesAutoresizingMaskIntoConstraints = false

        let content = NSView()
        content.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: content.leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: content.trailingAnchor),
            stack.topAnchor.constraint(equalTo: content.topAnchor),
            stack.bottomAnchor.constraint(lessThanOrEqualTo: content.bottomAnchor),
        ])
        window.contentView = content
    }

    func show() {
        refreshStatus(prompt: true)
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
        pollTimer = Timer.scheduledTimer(timeInterval: 1.0, target: self,
                                         selector: #selector(pollPermission),
                                         userInfo: nil, repeats: true)
    }

    func close() {
        pollTimer?.invalidate()
        pollTimer = nil
        window.orderOut(nil)
    }

    @objc private func pollPermission() { refreshStatus(prompt: false) }

    private func refreshStatus(prompt: Bool) {
        let key = kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String
        let trusted = AXIsProcessTrustedWithOptions([key: prompt] as CFDictionary)
        if trusted {
            statusLabel.stringValue = "✅ 已获得“辅助功能”权限，可以锁定整块键盘。"
            statusLabel.textColor = .systemGreen
            openSettingsButton.isHidden = true
            startButton.isEnabled = true
        } else {
            statusLabel.stringValue = "⚠️ 还没有“辅助功能”权限。请点下面按钮，在 系统设置 › 隐私与安全性 › 辅助功能 里打开“宝宝钢琴”的开关（没有就点 + 号添加），打开后这里会自动变绿。"
            statusLabel.textColor = .systemOrange
            openSettingsButton.isHidden = false
            startButton.isEnabled = false
        }
    }

    @objc private func openAccessibilitySettings() {
        if let url = URL(string: "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility") {
            NSWorkspace.shared.open(url)
        }
    }

    @objc private func volumeChanged(_ sender: NSSlider) {
        sound.setVolume(Float(sender.doubleValue))
    }

    @objc private func visualsToggled(_ sender: NSButton) {
        Settings.showVisuals = (sender.state == .on)
    }

    @objc private func testSound() {
        for (i, idx) in [0, 2, 4, 7].enumerated() {
            DispatchQueue.main.asyncAfter(deadline: .now() + Double(i) * 0.25) { [weak self] in
                self?.sound.play(index: idx)
            }
        }
    }

    @objc private func startPressed() { onStart?() }
}

// MARK: - App

final class AppDelegate: NSObject, NSApplicationDelegate {
    private let sound = PianoSound()
    private var setup: SetupController!
    private var kioskWindow: KioskWindow?
    private var playView: PlayView?
    private var activity: NSObjectProtocol?

    func applicationDidFinishLaunching(_ notification: Notification) {
        buildMenu()
        setup = SetupController(sound: sound)
        setup.onStart = { [weak self] in self?.startPlaying() }
        setup.show()
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }

    private func buildMenu() {
        let main = NSMenu()
        let appItem = NSMenuItem()
        main.addItem(appItem)
        let appMenu = NSMenu()
        appMenu.addItem(withTitle: "退出宝宝钢琴", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        appItem.submenu = appMenu
        NSApp.mainMenu = main
    }

    private func startPlaying() {
        guard KeyboardLock.shared.start() else {
            let alert = NSAlert()
            alert.messageText = "无法锁定键盘"
            alert.informativeText = "请确认已在 系统设置 › 隐私与安全性 › 辅助功能 中打开“宝宝钢琴”。如果刚重新编译过，请先把旧的条目删掉（−），再重新添加。"
            alert.runModal()
            return
        }
        setup.close()

        let screen = NSScreen.main ?? NSScreen.screens[0]
        let win = KioskWindow(contentRect: screen.frame, styleMask: .borderless,
                              backing: .buffered, defer: false)
        win.level = .screenSaver
        win.collectionBehavior = [.canJoinAllSpaces, .stationary, .ignoresCycle]
        win.isReleasedWhenClosed = false
        let view = PlayView(frame: NSRect(origin: .zero, size: screen.frame.size))
        win.contentView = view
        win.setFrame(screen.frame, display: true)
        kioskWindow = win
        playView = view

        win.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
        // 系统级“展台模式”：隐藏 Dock/菜单栏，禁止切换程序、强制退出、注销/关机对话框
        NSApp.presentationOptions = [
            .hideDock, .hideMenuBar,
            .disableProcessSwitching, .disableForceQuit,
            .disableSessionTermination, .disableHideApplication,
            .disableAppleMenu,
        ]
        win.makeFirstResponder(view)

        // 玩的时候屏幕不自动熄灭、不休眠：屏幕一旦熄灭就可能进入锁屏，
        // 锁屏密码框是“安全输入”，任何程序都拦不住，宝宝的按键会打进密码框。
        activity = ProcessInfo.processInfo.beginActivity(
            options: [.idleDisplaySleepDisabled, .idleSystemSleepDisabled, .userInitiated],
            reason: "Baby is playing piano")

        KeyboardLock.shared.onKeyDown = { [weak self] code in
            self?.handleKey(code)
        }
        view.onClickNote = { [weak self] point in
            guard let self = self else { return }
            let idx = Int.random(in: 0..<self.sound.scale.count)
            if self.sound.play(index: idx) {
                self.playView?.burst(noteIndex: idx, noteCount: self.sound.scale.count, at: point)
            }
        }
        view.onExit = {
            KeyboardLock.shared.stop()
            NSApp.presentationOptions = []
            NSApp.terminate(nil)
        }
    }

    private func handleKey(_ code: Int) {
        let count = sound.scale.count
        let idx = KeyMap.noteIndex(forKeyCode: code, scaleCount: count)
        if sound.play(index: idx) {
            playView?.burst(noteIndex: idx, noteCount: count)
        }
    }

    func applicationWillTerminate(_ notification: Notification) {
        KeyboardLock.shared.stop()
        if let a = activity { ProcessInfo.processInfo.endActivity(a) }
    }
}

let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.regular)
app.run()
