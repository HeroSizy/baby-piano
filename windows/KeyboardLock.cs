using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Runtime.InteropServices;

namespace BabyPiano
{
    /// <summary>
    /// 全局键盘锁：WH_KEYBOARD_LL 低级键盘钩子吞掉所有按键（含 Win 键、Alt+Tab、Alt+F4、Ctrl+Esc、
    /// 音量/媒体键），只拿来弹琴。可选 WH_MOUSE_LL 同时锁住鼠标/触控板。
    /// Ctrl+Alt+Del 由系统在钩子之前处理，任何普通程序都拦不住。
    /// </summary>
    internal sealed class KeyboardLock
    {
        /// <summary>按下一个新键（不含自动重复）：虚拟键码、扫描码、是否扩展键。</summary>
        public event Action<int, int, bool> KeyDown;

        private IntPtr _kbHook = IntPtr.Zero;
        private IntPtr _mouseHook = IntPtr.Zero;
        // 委托必须一直被引用，否则会被 GC 回收导致崩溃
        private readonly Native.HookProc _kbProc;
        private readonly Native.HookProc _mouseProc;
        private readonly HashSet<uint> _down = new HashSet<uint>();

        public bool IsActive => _kbHook != IntPtr.Zero;
        public bool MouseLocked => _mouseHook != IntPtr.Zero;
        /// <summary>最后一次收到钩子回调的时间（Stopwatch 计时）。</summary>
        public long LastCallback { get; private set; }

        public KeyboardLock()
        {
            _kbProc = KeyboardProc;
            _mouseProc = MouseProc;
        }

        public bool Start(bool lockMouse)
        {
            Stop();
            IntPtr hMod = Native.GetModuleHandle(Process.GetCurrentProcess().MainModule.ModuleName);
            _kbHook = Native.SetWindowsHookEx(Native.WH_KEYBOARD_LL, _kbProc, hMod, 0);
            if (_kbHook == IntPtr.Zero)
                _kbHook = Native.SetWindowsHookEx(Native.WH_KEYBOARD_LL, _kbProc, IntPtr.Zero, 0);
            if (_kbHook == IntPtr.Zero) return false;

            if (lockMouse)
            {
                _mouseHook = Native.SetWindowsHookEx(Native.WH_MOUSE_LL, _mouseProc, hMod, 0);
                if (_mouseHook == IntPtr.Zero)
                    _mouseHook = Native.SetWindowsHookEx(Native.WH_MOUSE_LL, _mouseProc, IntPtr.Zero, 0);
            }
            LastCallback = Stopwatch.GetTimestamp();
            return true;
        }

        /// <summary>
        /// 钩子回调超时（系统卡顿）时 Windows 会悄悄摘掉钩子。窗口一旦收到按键，就说明钩子失效了，
        /// 调用本方法重新挂上。
        /// </summary>
        public void Reinstall()
        {
            bool mouse = MouseLocked;
            Start(mouse);
        }

        public void Stop()
        {
            if (_kbHook != IntPtr.Zero) { Native.UnhookWindowsHookEx(_kbHook); _kbHook = IntPtr.Zero; }
            if (_mouseHook != IntPtr.Zero) { Native.UnhookWindowsHookEx(_mouseHook); _mouseHook = IntPtr.Zero; }
            _down.Clear();
        }

        private IntPtr KeyboardProc(int nCode, IntPtr wParam, IntPtr lParam)
        {
            if (nCode < 0) return Native.CallNextHookEx(_kbHook, nCode, wParam, lParam);
            LastCallback = Stopwatch.GetTimestamp();

            var k = (Native.KBDLLHOOKSTRUCT)Marshal.PtrToStructure(lParam, typeof(Native.KBDLLHOOKSTRUCT));
            int msg = wParam.ToInt32();
            if (msg == Native.WM_KEYDOWN || msg == Native.WM_SYSKEYDOWN)
            {
                // 按住不放时的自动重复不再出声
                if (_down.Add(k.vkCode))
                {
                    try { KeyDown?.Invoke((int)k.vkCode, (int)k.scanCode, (k.flags & Native.LLKHF_EXTENDED) != 0); }
                    catch { /* 回调里绝不能抛异常，否则钩子会被系统摘掉 */ }
                }
            }
            else if (msg == Native.WM_KEYUP || msg == Native.WM_SYSKEYUP)
            {
                _down.Remove(k.vkCode);
            }
            // 返回非 0 = 吞掉这个按键，系统和其他程序都收不到
            return (IntPtr)1;
        }

        private IntPtr MouseProc(int nCode, IntPtr wParam, IntPtr lParam)
        {
            if (nCode < 0) return Native.CallNextHookEx(_mouseHook, nCode, wParam, lParam);
            // 吞掉所有鼠标/触控板事件：光标不动，点击无效
            return (IntPtr)1;
        }
    }

    /// <summary>键位 → 音高：按物理行排列，左低右高，上排略高（与 macOS 版一致）。</summary>
    internal static class KeyMap
    {
        // 扫描码（Set 1），按 ANSI 键盘物理行排列（左 → 右）
        private static readonly int[][] Rows =
        {
            new[] { 0x29, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E }, // ` 1 2 … = Backspace
            new[] { 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x2B }, // Tab Q W … ] \
            new[] { 0x3A, 0x1E, 0x1F, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x1C },       // Caps A S … ' Enter
            new[] { 0x2A, 0x2C, 0x2D, 0x2E, 0x2F, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36 },             // Shift Z X … / RShift
        };

        private static readonly Dictionary<int, int> Table = Build();

        private static Dictionary<int, int> Build()
        {
            var t = new Dictionary<int, int>();
            for (int r = 0; r < Rows.Length; r++)
                for (int c = 0; c < Rows[r].Length; c++)
                {
                    int b = (int)Math.Round(c / 13.0 * 12.0, MidpointRounding.AwayFromZero);
                    t[Rows[r][c]] = Math.Min(15, Math.Max(0, b + (3 - r)));
                }
            return t;
        }

        public static int NoteIndex(int vk, int scanCode, bool extended, int scaleCount)
        {
            if (!extended && Table.TryGetValue(scanCode, out int i)) return Math.Min(i, scaleCount - 1);
            // 其他键（功能键、方向键、空格、小键盘、媒体键……）：固定但分散的映射
            return (vk * 7 + (extended ? 3 : 0)) % scaleCount;
        }
    }

    /// <summary>家长在键盘上依次输入 quitpiano（字母间隔不超过 3 秒）即可退出。</summary>
    internal sealed class ExitSequence
    {
        private const string Word = "quitpiano";
        private readonly System.Text.StringBuilder _buf = new System.Text.StringBuilder();
        private long _last;

        public bool Feed(int vk)
        {
            long now = Stopwatch.GetTimestamp();
            if (now - _last > 3 * Stopwatch.Frequency) _buf.Clear();
            _last = now;
            if (vk < 'A' || vk > 'Z')
            {
                _buf.Clear();   // 中间夹了别的键就重新来
                return false;
            }
            _buf.Append(char.ToLowerInvariant((char)vk));
            if (_buf.Length > Word.Length) _buf.Remove(0, _buf.Length - Word.Length);
            return _buf.ToString() == Word;
        }
    }
}
