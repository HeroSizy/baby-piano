using System;
using System.Drawing;
using System.Windows.Forms;

namespace BabyPiano
{
    /// <summary>家长设置页：调音量、试听、选项，然后“开始弹琴”。</summary>
    internal sealed class SetupForm : Form
    {
        public event Action StartRequested;

        private readonly PianoSound _sound;
        private readonly TrackBar _volume;
        private readonly Label _systemVolume;
        private readonly Timer _poll = new Timer { Interval = 1000 };

        public SetupForm(PianoSound sound)
        {
            _sound = sound;
            Text = "宝宝钢琴";
            AutoScaleMode = AutoScaleMode.Dpi;
            AutoScaleDimensions = new SizeF(96F, 96F);
            Font = new Font("Microsoft YaHei UI", 10f);
            FormBorderStyle = FormBorderStyle.FixedSingle;
            MaximizeBox = false;
            StartPosition = FormStartPosition.CenterScreen;
            AutoSize = true;
            AutoSizeMode = AutoSizeMode.GrowAndShrink;
            BackColor = SystemColors.Window;

            var stack = new FlowLayoutPanel
            {
                FlowDirection = FlowDirection.TopDown,
                WrapContents = false,
                AutoSize = true,
                AutoSizeMode = AutoSizeMode.GrowAndShrink,
                Padding = new Padding(28, 22, 28, 24),
            };
            Controls.Add(stack);

            Label L(string text, float size = 10f, bool bold = false, Color? color = null)
            {
                var l = new Label
                {
                    Text = text,
                    AutoSize = true,
                    MaximumSize = new Size(460, 0),
                    Font = new Font("Microsoft YaHei UI", size, bold ? FontStyle.Bold : FontStyle.Regular),
                    ForeColor = color ?? SystemColors.ControlText,
                    Margin = new Padding(0, 0, 0, 10),
                };
                stack.Controls.Add(l);
                return l;
            }

            L("宝宝钢琴", 20f, true);
            L("开始后整块键盘只会弹钢琴：Win 键、Alt+Tab、Alt+F4、Ctrl+Esc、音量键等都被拦下。" +
              "每个键固定一个音，都在五声音阶里，乱按也好听。默认黑屏，宝宝只听声音。");
            L("家长退出：鼠标在屏幕右上角按住 3 秒，或在键盘上连续输入 quitpiano。", 10f, true, Color.FromArgb(0, 102, 204));
            L("拦不住的：Ctrl+Alt+Del（会出现系统蓝色菜单，点“取消”即可）、电源键、合上笔记本。", 9f, false, SystemColors.GrayText);

            L("音量", 10f, true).Margin = new Padding(0, 8, 0, 2);
            var volRow = new FlowLayoutPanel { AutoSize = true, WrapContents = false, Margin = new Padding(0, 0, 0, 4) };
            _volume = new TrackBar
            {
                Minimum = 0,
                Maximum = 100,
                TickFrequency = 10,
                Width = 320,
                Value = (int)Math.Round(Settings.Volume * 100),
            };
            _volume.ValueChanged += (s, e) =>
            {
                float v = _volume.Value / 100f;
                _sound.Volume = v;
                Settings.Volume = v;
            };
            var test = new Button { Text = "试听", AutoSize = true, Margin = new Padding(8, 6, 0, 0) };
            test.Click += (s, e) => TestSound();
            volRow.Controls.Add(_volume);
            volRow.Controls.Add(test);
            stack.Controls.Add(volRow);
            _systemVolume = L("", 9f, false, SystemColors.GrayText);

            var mouse = new CheckBox
            {
                Text = "同时锁住鼠标和触控板（笔记本推荐；之后只能用键盘输入 quitpiano 退出）",
                Checked = Settings.LockMouse,
                AutoSize = true,
                MaximumSize = new Size(460, 0),
                Margin = new Padding(0, 6, 0, 6),
            };
            mouse.CheckedChanged += (s, e) => Settings.LockMouse = mouse.Checked;
            stack.Controls.Add(mouse);

            var winL = new CheckBox
            {
                Text = "防止 Win+L 锁屏（临时修改当前用户设置，退出时自动恢复）",
                Checked = Settings.BlockWinL,
                AutoSize = true,
                MaximumSize = new Size(460, 0),
                Margin = new Padding(0, 0, 0, 6),
            };
            winL.CheckedChanged += (s, e) => Settings.BlockWinL = winL.Checked;
            stack.Controls.Add(winL);

            var visuals = new CheckBox
            {
                Text = "显示彩色动画（默认黑屏）",
                Checked = Settings.ShowVisuals,
                AutoSize = true,
                Margin = new Padding(0, 0, 0, 14),
            };
            visuals.CheckedChanged += (s, e) => Settings.ShowVisuals = visuals.Checked;
            stack.Controls.Add(visuals);

            var start = new Button
            {
                Text = "开始弹琴（全屏锁定键盘）",
                Font = new Font("Microsoft YaHei UI", 12f, FontStyle.Bold),
                Size = new Size(460, 48),
                BackColor = Color.FromArgb(0, 120, 215),
                ForeColor = Color.White,
                FlatStyle = FlatStyle.Flat,
            };
            start.FlatAppearance.BorderSize = 0;
            start.Click += (s, e) => StartRequested?.Invoke();
            stack.Controls.Add(start);
            AcceptButton = start;

            _poll.Tick += (s, e) => RefreshSystemVolume();
            _poll.Start();
            RefreshSystemVolume();
        }

        private void RefreshSystemVolume()
        {
            var v = SystemVolume.Get();
            _systemVolume.Text = v.HasValue
                ? $"Windows 系统音量现在是 {Math.Round(v.Value * 100)}%，弹琴时最高 {Math.Round(SystemVolume.MaxSystemVolume * 100)}%（开始后音量键会被锁住）。"
                : $"弹琴时 Windows 系统音量最高 {Math.Round(SystemVolume.MaxSystemVolume * 100)}%（开始后音量键会被锁住）。";
        }

        private void TestSound()
        {
            // 和开始后一样：先把系统音量压到上限以下，再弹三个音
            SystemVolume.EnforceCap();
            RefreshSystemVolume();
            int[] seq = { 5, 7, 9 };
            int i = 0;
            var t = new Timer { Interval = 260 };
            t.Tick += (s, e) =>
            {
                _sound.Play(seq[i++]);
                if (i >= seq.Length) { t.Stop(); t.Dispose(); }
            };
            _sound.Play(seq[i++]);
            t.Start();
        }

        protected override void Dispose(bool disposing)
        {
            if (disposing) _poll.Dispose();
            base.Dispose(disposing);
        }
    }
}
