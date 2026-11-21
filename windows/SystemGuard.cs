using System;
using System.Collections.Generic;
using System.IO;
using System.Runtime.InteropServices;
using Microsoft.Win32;

namespace BabyPiano
{
    /// <summary>设置，存在 %APPDATA%\BabyPiano\settings.ini。</summary>
    internal static class Settings
    {
        private static readonly string Dir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "BabyPiano");
        private static readonly string FilePath = Path.Combine(Dir, "settings.ini");
        private static readonly Dictionary<string, string> Values = Load();

        public static float Volume
        {
            get => float.TryParse(Get("volume"), System.Globalization.NumberStyles.Float,
                System.Globalization.CultureInfo.InvariantCulture, out var v) ? Math.Max(0, Math.Min(1, v)) : 0.45f;
            set => Put("volume", value.ToString("0.###", System.Globalization.CultureInfo.InvariantCulture));
        }

        public static bool LockMouse { get => Get("lockMouse") == "1"; set => Put("lockMouse", value ? "1" : "0"); }
        public static bool ShowVisuals { get => Get("showVisuals") == "1"; set => Put("showVisuals", value ? "1" : "0"); }
        /// <summary>默认开启：防止 Win+L 锁屏（宝宝按键会打进密码框）。</summary>
        public static bool BlockWinL { get => Get("blockWinL") != "0"; set => Put("blockWinL", value ? "1" : "0"); }

        /// <summary>
        /// 进入弹琴前 DisableLockWorkstation 的原值（"absent" 或数字）。
        /// 程序意外退出时留在文件里，下次启动据此恢复。
        /// </summary>
        public static string PendingLockPolicyRestore
        {
            get => Get("pendingLockPolicyRestore");
            set => Put("pendingLockPolicyRestore", value);
        }

        private static string Get(string key) => Values.TryGetValue(key, out var v) ? v : null;

        private static void Put(string key, string value)
        {
            if (value == null) Values.Remove(key); else Values[key] = value;
            try
            {
                Directory.CreateDirectory(Dir);
                var lines = new List<string>();
                foreach (var kv in Values) lines.Add(kv.Key + "=" + kv.Value);
                File.WriteAllLines(FilePath, lines);
            }
            catch { /* 保存失败不影响使用 */ }
        }

        private static Dictionary<string, string> Load()
        {
            var d = new Dictionary<string, string>();
            try
            {
                if (File.Exists(FilePath))
                    foreach (var line in File.ReadAllLines(FilePath))
                    {
                        int i = line.IndexOf('=');
                        if (i > 0) d[line.Substring(0, i).Trim()] = line.Substring(i + 1).Trim();
                    }
            }
            catch { }
            return d;
        }
    }

    /// <summary>
    /// 弹琴期间对系统做的临时调整，退出时全部恢复：
    /// 防休眠/熄屏、关掉“连按 5 次 Shift”等辅助功能快捷键、（可选）禁用 Win+L 锁屏。
    /// </summary>
    internal sealed class SystemGuard
    {
        private Native.STICKYKEYS _sk;
        private Native.TOGGLEKEYS _tk;
        private Native.FILTERKEYS _fk;
        private bool _accessibilitySaved;
        private bool _lockPolicyChanged;

        private const string PolicyKey = @"Software\Microsoft\Windows\CurrentVersion\Policies\System";
        private const string PolicyValue = "DisableLockWorkstation";

        public bool WinLBlocked => _lockPolicyChanged;

        public void Engage(bool blockWinL)
        {
            // 屏幕不熄灭、电脑不睡眠：熄屏后可能进入锁屏，宝宝的按键会打进密码框
            Native.SetThreadExecutionState(Native.EXECUTION_STATE.ES_CONTINUOUS |
                                           Native.EXECUTION_STATE.ES_DISPLAY_REQUIRED |
                                           Native.EXECUTION_STATE.ES_SYSTEM_REQUIRED);
            DisableAccessibilityShortcuts();
            if (blockWinL) DisableLockWorkstation();
        }

        public void Release()
        {
            Native.SetThreadExecutionState(Native.EXECUTION_STATE.ES_CONTINUOUS);
            RestoreAccessibilityShortcuts();
            RestoreLockWorkstation();
        }

        // ---------- 粘滞键 / 筛选键 / 切换键的快捷方式 ----------
        // 只在本次登录内生效（不写入用户配置），即使程序崩溃，注销后也会恢复。

        private void DisableAccessibilityShortcuts()
        {
            _sk = new Native.STICKYKEYS { cbSize = (uint)Marshal.SizeOf(typeof(Native.STICKYKEYS)) };
            _tk = new Native.TOGGLEKEYS { cbSize = (uint)Marshal.SizeOf(typeof(Native.TOGGLEKEYS)) };
            _fk = new Native.FILTERKEYS { cbSize = (uint)Marshal.SizeOf(typeof(Native.FILTERKEYS)) };
            bool ok = Native.SystemParametersInfo(Native.SPI_GETSTICKYKEYS, _sk.cbSize, ref _sk, 0)
                    & Native.SystemParametersInfo(Native.SPI_GETTOGGLEKEYS, _tk.cbSize, ref _tk, 0)
                    & Native.SystemParametersInfo(Native.SPI_GETFILTERKEYS, _fk.cbSize, ref _fk, 0);
            if (!ok) return;
            _accessibilitySaved = true;

            // 已经在用这些功能的家长：保持不动
            if ((_sk.dwFlags & Native.SKF_STICKYKEYSON) == 0)
            {
                var s = _sk;
                s.dwFlags &= ~(Native.SKF_HOTKEYACTIVE | Native.SKF_CONFIRMHOTKEY);
                Native.SystemParametersInfo(Native.SPI_SETSTICKYKEYS, s.cbSize, ref s, 0);
            }
            if ((_tk.dwFlags & Native.TKF_TOGGLEKEYSON) == 0)
            {
                var t = _tk;
                t.dwFlags &= ~(Native.TKF_HOTKEYACTIVE | Native.TKF_CONFIRMHOTKEY);
                Native.SystemParametersInfo(Native.SPI_SETTOGGLEKEYS, t.cbSize, ref t, 0);
            }
            if ((_fk.dwFlags & Native.FKF_FILTERKEYSON) == 0)
            {
                var f = _fk;
                f.dwFlags &= ~(Native.FKF_HOTKEYACTIVE | Native.FKF_CONFIRMHOTKEY);
                Native.SystemParametersInfo(Native.SPI_SETFILTERKEYS, f.cbSize, ref f, 0);
            }
        }

        private void RestoreAccessibilityShortcuts()
        {
            if (!_accessibilitySaved) return;
            _accessibilitySaved = false;
            Native.SystemParametersInfo(Native.SPI_SETSTICKYKEYS, _sk.cbSize, ref _sk, 0);
            Native.SystemParametersInfo(Native.SPI_SETTOGGLEKEYS, _tk.cbSize, ref _tk, 0);
            Native.SystemParametersInfo(Native.SPI_SETFILTERKEYS, _fk.cbSize, ref _fk, 0);
        }

        // ---------- Win+L ----------
        // Win+L 由系统在键盘钩子之前处理，只能通过当前用户的策略 DisableLockWorkstation 临时关掉。

        private void DisableLockWorkstation()
        {
            try
            {
                using (var key = Registry.CurrentUser.CreateSubKey(PolicyKey))
                {
                    object old = key.GetValue(PolicyValue);
                    if (old is int i && i == 1) return;   // 本来就禁用了（可能是公司策略），不碰
                    Settings.PendingLockPolicyRestore = old == null ? "absent" : Convert.ToString(old);
                    key.SetValue(PolicyValue, 1, RegistryValueKind.DWord);
                    _lockPolicyChanged = true;
                }
            }
            catch
            {
                Settings.PendingLockPolicyRestore = null;   // 没有权限写：放弃，Win+L 仍可锁屏
            }
        }

        private void RestoreLockWorkstation()
        {
            if (!_lockPolicyChanged) return;
            _lockPolicyChanged = false;
            RestorePendingLockPolicy();
        }

        /// <summary>按设置文件里记下的原值恢复（也用于上次程序意外退出后的启动时）。</summary>
        public static void RestorePendingLockPolicy()
        {
            string pending = Settings.PendingLockPolicyRestore;
            if (string.IsNullOrEmpty(pending)) return;
            try
            {
                using (var key = Registry.CurrentUser.CreateSubKey(PolicyKey))
                {
                    if (pending == "absent") key.DeleteValue(PolicyValue, false);
                    else if (int.TryParse(pending, out int v)) key.SetValue(PolicyValue, v, RegistryValueKind.DWord);
                }
                Settings.PendingLockPolicyRestore = null;
            }
            catch { }
        }
    }
}
