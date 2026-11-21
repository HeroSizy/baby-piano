using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Windows.Forms;

namespace BabyPiano
{
    /// <summary>
    /// 全屏、置顶、无边框的弹琴窗口。主屏一个，其他显示器各盖一个黑窗口。
    /// 也是第二道防线：键盘钩子万一失效，按键会落到这里，被吞掉并照样弹琴。
    /// </summary>
    internal sealed class PlayForm : Form
    {
        public event Action<int, int, bool> FallbackKey;
        public event Action<Point> ClickNote;
        public event Action ExitRequested;

        public bool AllowClose;
        private readonly bool _primary;
        private readonly bool _visuals;

        private const int ExitSize = 90;
        private const double ExitHoldSeconds = 3.0;
        private Stopwatch _hold;
        private readonly Timer _tick = new Timer { Interval = 30 };

        private sealed class Bubble
        {
            public PointF Center;
            public float Size;
            public Color Color;
            public Stopwatch Age;
        }
        private readonly List<Bubble> _bursts = new List<Bubble>();
        private readonly Random _rng = new Random();

        public PlayForm(Screen screen, bool primary, bool visuals)
        {
            _primary = primary;
            _visuals = visuals && primary;
            Text = "宝宝钢琴";
            FormBorderStyle = FormBorderStyle.None;
            StartPosition = FormStartPosition.Manual;
            Bounds = screen.Bounds;
            TopMost = true;
            ShowInTaskbar = primary;
            KeyPreview = true;
            BackColor = _visuals ? Color.FromArgb(18, 20, 43) : Color.Black;
            Cursor = Cursors.Default;
            SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer |
                     ControlStyles.UserPaint | ControlStyles.ResizeRedraw, true);
            _tick.Tick += (s, e) => OnTick();
        }

        protected override CreateParams CreateParams
        {
            get
            {
                var cp = base.CreateParams;
                if (!_primary) cp.ExStyle |= 0x08000000; // WS_EX_NOACTIVATE：副屏窗口不抢焦点
                return cp;
            }
        }

        // ---------- 键盘兜底：吞掉一切，不响提示音、不触发菜单 ----------

        protected override bool ProcessCmdKey(ref Message msg, Keys keyData)
        {
            const int WM_KEYDOWN = 0x100, WM_SYSKEYDOWN = 0x104;
            if (msg.Msg == WM_KEYDOWN || msg.Msg == WM_SYSKEYDOWN)
            {
                long lp = msg.LParam.ToInt64();
                bool repeat = (lp & (1L << 30)) != 0;
                if (!repeat)
                {
                    int scan = (int)((lp >> 16) & 0xFF);
                    bool ext = (lp & (1L << 24)) != 0;
                    FallbackKey?.Invoke((int)(keyData & Keys.KeyCode), scan, ext);
                }
            }
            return true;
        }

        protected override bool ProcessDialogKey(Keys keyData) => true;
        protected override void OnKeyDown(KeyEventArgs e) { e.Handled = true; e.SuppressKeyPress = true; }
        protected override void OnKeyUp(KeyEventArgs e) { e.Handled = true; }
        protected override void OnKeyPress(KeyPressEventArgs e) { e.Handled = true; }

        protected override void WndProc(ref Message m)
        {
            const int WM_SYSCOMMAND = 0x0112;
            const int SC_CLOSE = 0xF060, SC_KEYMENU = 0xF100, SC_SCREENSAVE = 0xF140, SC_MONITORPOWER = 0xF170,
                      SC_MINIMIZE = 0xF020, SC_TASKLIST = 0xF130;
            if (m.Msg == WM_SYSCOMMAND)
            {
                int cmd = m.WParam.ToInt32() & 0xFFF0;
                if (cmd == SC_CLOSE || cmd == SC_KEYMENU || cmd == SC_SCREENSAVE || cmd == SC_MONITORPOWER ||
                    cmd == SC_MINIMIZE || cmd == SC_TASKLIST)
                    return;   // Alt+F4、Alt 菜单、屏保、关显示器、最小化：一律不理
            }
            base.WndProc(ref m);
        }

        protected override void OnFormClosing(FormClosingEventArgs e)
        {
            // 只有家长退出或系统关机才能关闭
            if (!AllowClose && e.CloseReason != CloseReason.WindowsShutDown && e.CloseReason != CloseReason.TaskManagerClosing)
                e.Cancel = true;
            base.OnFormClosing(e);
        }

        // ---------- 鼠标：右上角长按退出，其他地方点一下也会响 ----------

        private Rectangle ExitRect => new Rectangle(ClientSize.Width - Scale(ExitSize), 0, Scale(ExitSize), Scale(ExitSize));
        private int Scale(int v) => (int)Math.Round(v * DeviceDpi / 96.0);

        protected override void OnMouseDown(MouseEventArgs e)
        {
            base.OnMouseDown(e);
            if (!_primary) return;
            if (ExitRect.Contains(e.Location))
            {
                _hold = Stopwatch.StartNew();
                _tick.Start();
            }
            else
            {
                ClickNote?.Invoke(e.Location);
            }
        }

        protected override void OnMouseMove(MouseEventArgs e)
        {
            base.OnMouseMove(e);
            if (_hold != null && !ExitRect.Contains(e.Location)) CancelHold();
        }

        protected override void OnMouseUp(MouseEventArgs e)
        {
            base.OnMouseUp(e);
            CancelHold();
        }

        private void CancelHold()
        {
            if (_hold == null) return;
            _hold = null;
            Invalidate(ExitRect);
        }

        private void OnTick()
        {
            if (_hold != null && _hold.Elapsed.TotalSeconds >= ExitHoldSeconds)
            {
                _hold = null;
                ExitRequested?.Invoke();
                return;
            }
            _bursts.RemoveAll(b => b.Age.Elapsed.TotalSeconds > 1.8);
            if (_hold == null && _bursts.Count == 0) _tick.Stop();
            Invalidate();
        }

        // ---------- 彩色圆圈（可选） ----------

        public void Burst(int noteIndex, int noteCount, Point? at = null)
        {
            if (!_visuals || _bursts.Count > 60) return;
            float t = noteIndex / (float)Math.Max(1, noteCount - 1);
            var w = ClientSize.Width; var h = ClientSize.Height;
            var center = at.HasValue
                ? new PointF(at.Value.X, at.Value.Y)
                : new PointF(w * (0.08f + 0.84f * t) + _rng.Next(-50, 51), h * (0.15f + 0.7f * (float)_rng.NextDouble()));
            _bursts.Add(new Bubble
            {
                Center = center,
                Size = Scale(_rng.Next(150, 281)),
                Color = FromHsv(t * 0.85 * 360, 0.55, 1.0),
                Age = Stopwatch.StartNew(),
            });
            _tick.Start();
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            base.OnPaint(e);
            var g = e.Graphics;
            g.SmoothingMode = SmoothingMode.AntiAlias;

            foreach (var b in _bursts)
            {
                double p = Math.Min(1, b.Age.Elapsed.TotalSeconds / 1.8);
                double ease = 1 - Math.Pow(1 - p, 3);
                double scale = 0.25 + (1.35 - 0.25) * ease;
                double alpha = p < 0.15 ? p / 0.15 * 0.85 : 0.85 * (1 - (p - 0.15) / 0.85);
                float r = (float)(b.Size * scale / 2);
                using (var br = new SolidBrush(Color.FromArgb((int)(Math.Max(0, alpha) * 255), b.Color)))
                    g.FillEllipse(br, b.Center.X - r, b.Center.Y - r, r * 2, r * 2);
            }

            if (_primary)
            {
                var er = ExitRect;
                var ring = Rectangle.Inflate(er, -Scale(25), -Scale(25));
                using (var track = new Pen(Color.FromArgb(30, 255, 255, 255), Scale(4)))
                    g.DrawEllipse(track, ring);
                if (_hold != null)
                {
                    float sweep = (float)Math.Min(360, _hold.Elapsed.TotalSeconds / ExitHoldSeconds * 360);
                    using (var pen = new Pen(Color.FromArgb(217, 255, 255, 255), Scale(4)) { StartCap = LineCap.Round, EndCap = LineCap.Round })
                        g.DrawArc(pen, ring, -90, sweep);
                }
                using (var font = new Font("Microsoft YaHei UI", 8.5f))
                using (var br = new SolidBrush(Color.FromArgb(64, 255, 255, 255)))
                {
                    const string hint = "家长：鼠标在右上角长按 3 秒退出，或键盘输入 quitpiano";
                    var size = g.MeasureString(hint, font);
                    g.DrawString(hint, font, br, ClientSize.Width - size.Width - Scale(15), er.Bottom + Scale(2));
                }
            }
        }

        private static Color FromHsv(double h, double s, double v)
        {
            h = (h % 360 + 360) % 360;
            double c = v * s, x = c * (1 - Math.Abs(h / 60 % 2 - 1)), m = v - c;
            double r, g, b;
            if (h < 60) { r = c; g = x; b = 0; }
            else if (h < 120) { r = x; g = c; b = 0; }
            else if (h < 180) { r = 0; g = c; b = x; }
            else if (h < 240) { r = 0; g = x; b = c; }
            else if (h < 300) { r = x; g = 0; b = c; }
            else { r = c; g = 0; b = x; }
            return Color.FromArgb((int)((r + m) * 255), (int)((g + m) * 255), (int)((b + m) * 255));
        }

        protected override void Dispose(bool disposing)
        {
            if (disposing) _tick.Dispose();
            base.Dispose(disposing);
        }
    }
}
