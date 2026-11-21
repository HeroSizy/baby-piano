using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Threading;
using System.Windows.Forms;

namespace BabyPiano
{
    internal static class Program
    {
        [STAThread]
        private static void Main()
        {
            using (var mutex = new Mutex(true, "BabyPiano.SingleInstance", out bool first))
            {
                if (!first) return;   // 已经开着一个了

                // 上次如果意外退出，先把 Win+L 设置恢复
                SystemGuard.RestorePendingLockPolicy();

                Application.EnableVisualStyles();
                Application.SetCompatibleTextRenderingDefault(false);
                using (var app = new App())
                    Application.Run(app.Setup);
            }
        }
    }

    /// <summary>程序主体：设置页 → 弹琴（全屏 + 键盘锁）→ 家长退出。</summary>
    internal sealed class App : IDisposable
    {
        public readonly SetupForm Setup;
        private readonly PianoSound _sound;
        private readonly KeyboardLock _lock = new KeyboardLock();
        private readonly SystemGuard _guard = new SystemGuard();
        private readonly ExitSequence _exitSequence = new ExitSequence();
        private readonly List<PlayForm> _forms = new List<PlayForm>();
        private readonly System.Windows.Forms.Timer _watchdog = new System.Windows.Forms.Timer { Interval = 1000 };
        private readonly Random _rng = new Random();
        private PlayForm _main;
        private bool _playing;
        private bool _exiting;

        public App()
        {
            _sound = new PianoSound(Settings.Volume);
            Setup = new SetupForm(_sound);
            Setup.StartRequested += StartPlaying;
            _lock.KeyDown += HandleKey;
            _watchdog.Tick += (s, e) => WatchdogTick();
            Application.ApplicationExit += (s, e) => Cleanup();
            AppDomain.CurrentDomain.UnhandledException += (s, e) => Cleanup();
            Application.ThreadException += (s, e) => { Cleanup(); Application.Exit(); };
        }

        private void StartPlaying()
        {
            if (_playing) return;
            if (!_lock.Start(Settings.LockMouse))
            {
                MessageBox.Show(Setup, "无法锁定键盘（键盘钩子安装失败）。请重启电脑后再试；如果装了安全软件，请允许“宝宝钢琴”运行。",
                    "宝宝钢琴", MessageBoxButtons.OK, MessageBoxIcon.Warning);
                return;
            }
            _playing = true;
            _guard.Engage(Settings.BlockWinL);
            SystemVolume.EnforceCap();

            // 主屏弹琴窗口；其他显示器盖上黑色窗口，防止鼠标点到别的程序
            foreach (var screen in Screen.AllScreens)
            {
                bool primary = screen.Primary;
                var f = new PlayForm(screen, primary, Settings.ShowVisuals);
                f.FallbackKey += (vk, scan, ext) =>
                {
                    // 钩子正常时窗口收不到按键；收到了说明钩子被系统摘掉了，立刻重新挂上
                    _lock.Reinstall();
                    HandleKey(vk, scan, ext);
                };
                if (primary)
                {
                    _main = f;
                    f.ClickNote += p =>
                    {
                        int idx = _rng.Next(_sound.Scale.Length);
                        if (_sound.Play(idx)) _main.Burst(idx, _sound.Scale.Length, p);
                    };
                    f.ExitRequested += ExitByParent;
                }
                _forms.Add(f);
            }
            if (_main == null) _main = _forms[0];

            foreach (var f in _forms) if (f != _main) f.Show();
            _main.Show();
            Setup.Hide();
            BringToFront();

            if (_lock.MouseLocked)
            {
                // 光标停在主屏中央（鼠标事件已全部吞掉，这里只是让它别停在屏幕边缘）
                Cursor.Position = new System.Drawing.Point(_main.Left + _main.Width / 2, _main.Top + _main.Height / 2);
            }
            _watchdog.Start();
        }

        private void HandleKey(int vk, int scan, bool extended)
        {
            if (!_playing) return;
            if (_exitSequence.Feed(vk))
            {
                // 退出放到消息循环里做，不在钩子回调里摘钩子
                _main.BeginInvoke(new Action(ExitByParent));
                return;
            }
            int count = _sound.Scale.Length;
            int idx = KeyMap.NoteIndex(vk, scan, extended, count);
            if (_sound.Play(idx)) _main?.Burst(idx, count);
        }

        /// <summary>每秒：系统音量不超上限；窗口保持在最前面。</summary>
        private void WatchdogTick()
        {
            if (!_playing) return;
            SystemVolume.EnforceCap();
            var fg = Native.GetForegroundWindow();
            bool ours = false;
            foreach (var f in _forms) if (f.IsHandleCreated && f.Handle == fg) { ours = true; break; }
            if (!ours) BringToFront();
            // 钩子超过 30 秒没有任何回调，保险起见重新挂一次（宝宝没在按时无影响）
            if (Stopwatch.GetTimestamp() - _lock.LastCallback > 30 * Stopwatch.Frequency) _lock.Reinstall();
        }

        private void BringToFront()
        {
            if (_main == null) return;
            foreach (var f in _forms) { f.TopMost = true; }
            IntPtr fg = Native.GetForegroundWindow();
            uint fgThread = Native.GetWindowThreadProcessId(fg, out _);
            uint me = Native.GetCurrentThreadId();
            bool attached = fgThread != 0 && fgThread != me && Native.AttachThreadInput(me, fgThread, true);
            try
            {
                Native.BringWindowToTop(_main.Handle);
                Native.SetForegroundWindow(_main.Handle);
                _main.Activate();
                _main.Focus();
            }
            finally
            {
                if (attached) Native.AttachThreadInput(me, fgThread, false);
            }
        }

        private void ExitByParent()
        {
            if (_exiting) return;
            _exiting = true;
            Cleanup();
            foreach (var f in _forms) { f.AllowClose = true; f.Close(); }
            Application.Exit();
        }

        private void Cleanup()
        {
            _watchdog.Stop();
            _lock.Stop();
            if (_playing)
            {
                _playing = false;
                _guard.Release();
            }
        }

        public void Dispose()
        {
            Cleanup();
            _sound.Dispose();
            _watchdog.Dispose();
        }
    }
}
