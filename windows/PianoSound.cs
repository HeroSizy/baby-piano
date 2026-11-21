using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Threading;

namespace BabyPiano
{
    /// <summary>
    /// 合成钢琴音 + 实时混音 + 限幅器，经 waveOut 输出。
    /// 不依赖任何音色文件：每个音在启动时用加法合成预先算好（带琴弦不谐和性、双弦拍频、击弦噪声）。
    /// </summary>
    internal sealed class PianoSound : IDisposable
    {
        public const int SampleRate = 44100;

        /// <summary>C 大调五声音阶 C3 ~ C6（MIDI 48..84，共 16 个音），任意组合都和谐。</summary>
        public readonly int[] Scale;

        private readonly float[][] _notes;
        private readonly List<Voice> _voices = new List<Voice>();
        private readonly object _lock = new object();
        private readonly Random _rng = new Random();
        private readonly Queue<long> _recentHits = new Queue<long>();

        private volatile float _volume;
        private Thread _thread;
        private volatile bool _running;

        // 限幅器状态（仅音频线程使用）
        private float _env;
        private const float LimiterThreshold = 0.55f;   // 输出峰值上限约 -5 dBFS

        private const int MaxVoices = 24;
        private const double NoteSeconds = 1.8;          // 与 macOS 版一致：1.8 秒后松键
        private const double ReleaseSeconds = 0.45;

        private sealed class Voice
        {
            public float[] Data;
            public int Pos;
            public float Gain;
            public int FadeLeft = -1;   // >=0 表示正在被抢占淡出
        }

        public PianoSound(float volume) : this(volume, true) { }

        /// <param name="startOutput">false = 不打开声卡（仅供测试离线渲染）。</param>
        internal PianoSound(float volume, bool startOutput)
        {
            _volume = volume;
            var notes = new List<int>();
            int[] pentatonic = { 0, 2, 4, 7, 9 };
            for (int octave = 0; octave < 3; octave++)
                foreach (var p in pentatonic) notes.Add(48 + octave * 12 + p);
            notes.Add(84);
            Scale = notes.ToArray();

            _notes = new float[Scale.Length][];
            for (int i = 0; i < Scale.Length; i++)
                _notes[i] = Synthesize(Scale[i], i, Scale.Length);

            if (!startOutput) return;
            _running = true;
            _thread = new Thread(AudioLoop) { IsBackground = true, Name = "BabyPiano audio", Priority = ThreadPriority.Highest };
            _thread.Start();
        }

        public float Volume
        {
            get => _volume;
            set => _volume = Math.Max(0f, Math.Min(1f, value));
        }

        /// <summary>播放音阶中第 index 个音。返回 false 表示被限流（宝宝一巴掌按下十几个键时）。</summary>
        public bool Play(int index)
        {
            long now = Stopwatch.GetTimestamp();
            long window = (long)(0.4 * Stopwatch.Frequency);
            lock (_lock)
            {
                // 最多约每秒 12 个音：0.4 秒内最多 5 个
                while (_recentHits.Count > 0 && now - _recentHits.Peek() >= window) _recentHits.Dequeue();
                if (_recentHits.Count >= 5) return false;
                _recentHits.Enqueue(now);

                index = Math.Max(0, Math.Min(_notes.Length - 1, index));
                // 中等力度，略带随机（对应 macOS 版 velocity 58...72）
                float gain = 0.40f * (0.9f + 0.2f * (float)_rng.NextDouble());

                if (_voices.Count >= MaxVoices)
                {
                    // 抢占最老且还没在淡出的声部：5 ms 淡出，避免“咔”声
                    foreach (var v in _voices)
                        if (v.FadeLeft < 0) { v.FadeLeft = SampleRate / 200; break; }
                }
                _voices.Add(new Voice { Data = _notes[index], Gain = gain });
            }
            return true;
        }

        // ------------------------------------------------------------------
        // 合成
        // ------------------------------------------------------------------

        private static float[] Synthesize(int midi, int index, int count)
        {
            double f0 = 440.0 * Math.Pow(2, (midi - 69) / 12.0);
            int total = (int)((NoteSeconds + ReleaseSeconds) * SampleRate);
            var buf = new double[total];

            const double B = 0.00035;                 // 琴弦不谐和系数
            double baseDecay = 0.55 + f0 / 450.0;     // 高音衰减更快
            var rng = new Random(midi * 7919);

            for (int n = 1; n <= 12; n++)
            {
                double fn = n * f0 * Math.Sqrt(1 + B * n * n);
                if (fn > SampleRate * 0.42) break;
                double amp = Math.Pow(n, -1.25) * (n == 1 ? 1.0 : 0.85);
                if (n == 7 || n == 9) amp *= 0.4;     // 击弦点约在 1/7～1/9 处，削弱这两个泛音
                double dFast = baseDecay * (2.6 + 0.55 * (n - 1));
                double dSlow = baseDecay * (0.45 + 0.22 * (n - 1));
                double detune = 1 + (0.00045 + 0.0001 * rng.NextDouble());   // 双弦微失谐 → 自然的拍频
                double ph1 = rng.NextDouble() * 2 * Math.PI, ph2 = rng.NextDouble() * 2 * Math.PI;
                double w1 = 2 * Math.PI * fn / SampleRate, w2 = w1 * detune;
                // 用复数旋转代替逐点 sin/exp，启动更快
                double c1 = Math.Cos(w1), s1 = Math.Sin(w1), c2 = Math.Cos(w2), s2 = Math.Sin(w2);
                double x1 = Math.Cos(ph1), y1 = Math.Sin(ph1), x2 = Math.Cos(ph2), y2 = Math.Sin(ph2);
                double kFast = Math.Exp(-dFast / SampleRate), kSlow = Math.Exp(-dSlow / SampleRate);
                double eFast = 0.65 * amp * 0.5, eSlow = 0.35 * amp * 0.5;
                for (int i = 0; i < total; i++)
                {
                    buf[i] += (eFast + eSlow) * (y1 + y2);
                    eFast *= kFast; eSlow *= kSlow;
                    double nx1 = x1 * c1 - y1 * s1; y1 = x1 * s1 + y1 * c1; x1 = nx1;
                    double nx2 = x2 * c2 - y2 * s2; y2 = x2 * s2 + y2 * c2; x2 = nx2;
                    if ((i & 4095) == 4095)
                    {
                        // 定期归一化，防止长时间累积误差
                        double m1 = 1 / Math.Sqrt(x1 * x1 + y1 * y1), m2 = 1 / Math.Sqrt(x2 * x2 + y2 * y2);
                        x1 *= m1; y1 *= m1; x2 *= m2; y2 *= m2;
                    }
                }
            }

            // 击弦噪声：极短、经低通，增加“敲击感”
            double lp = 0;
            int noiseLen = SampleRate / 80;
            double cutoff = Math.Min(0.5, f0 * 6 / SampleRate * 2 * Math.PI);
            for (int i = 0; i < noiseLen; i++)
            {
                double nz = rng.NextDouble() * 2 - 1;
                lp += cutoff * (nz - lp);
                double e = Math.Exp(-(double)i / (noiseLen / 4.0));
                buf[i] += 0.12 * lp * e;
            }

            // 起音（2 ms）与松键后的释音
            int attack = SampleRate / 500;
            int releaseStart = (int)(NoteSeconds * SampleRate);
            double peak = 1e-9;
            for (int i = 0; i < total; i++)
            {
                double g = 1;
                if (i < attack) g *= (double)i / attack;
                if (i >= releaseStart)
                {
                    double r = (double)(i - releaseStart) / (ReleaseSeconds * SampleRate);
                    g *= Math.Exp(-6.0 * r) * (1 - r);   // 平滑落到 0
                }
                buf[i] *= g;
                peak = Math.Max(peak, Math.Abs(buf[i]));
            }

            // 归一化；低音略响、高音略轻，听感更均衡
            double loud = 1.0 - 0.3 * index / Math.Max(1, count - 1);
            var result = new float[total];
            for (int i = 0; i < total; i++) result[i] = (float)(buf[i] / peak * loud);
            return result;
        }

        // ------------------------------------------------------------------
        // 混音 + 限幅
        // ------------------------------------------------------------------

        /// <summary>把下一段混音写入 output（-1..1）。音频线程调用；也供测试使用。</summary>
        internal void Render(float[] output)
        {
            Array.Clear(output, 0, output.Length);
            float vol = _volume;
            lock (_lock)
            {
                for (int k = _voices.Count - 1; k >= 0; k--)
                {
                    var v = _voices[k];
                    int n = Math.Min(output.Length, v.Data.Length - v.Pos);
                    for (int i = 0; i < n; i++)
                    {
                        float g = v.Gain;
                        if (v.FadeLeft >= 0)
                        {
                            if (v.FadeLeft == 0) { n = i; break; }
                            g *= v.FadeLeft / (float)(SampleRate / 200);
                            v.FadeLeft--;
                        }
                        output[i] += v.Data[v.Pos + i] * g;
                    }
                    v.Pos += n;
                    if (v.Pos >= v.Data.Length || v.FadeLeft == 0) _voices.RemoveAt(k);
                }
            }

            // 峰值限幅：瞬时压下、约 150 ms 回升。输出峰值永远不超过 LimiterThreshold。
            float rel = (float)Math.Exp(-1.0 / (0.15 * SampleRate));
            for (int i = 0; i < output.Length; i++)
            {
                float x = output[i] * vol;
                float a = Math.Abs(x);
                _env = a > _env ? a : _env * rel + a * (1 - rel);
                if (_env > LimiterThreshold) x *= LimiterThreshold / _env;
                output[i] = x;
            }
        }

        private void AudioLoop()
        {
            const int bufSamples = 882;   // 20 ms
            const int bufCount = 4;
            var mix = new float[bufSamples];
            var pcm = new short[bufSamples];
            int hdrSize = Marshal.SizeOf(typeof(Native.WAVEHDR));

            while (_running)
            {
                var evt = new AutoResetEvent(false);
                var fmt = new Native.WAVEFORMATEX
                {
                    wFormatTag = Native.WAVE_FORMAT_PCM,
                    nChannels = 1,
                    nSamplesPerSec = SampleRate,
                    wBitsPerSample = 16,
                    nBlockAlign = 2,
                    nAvgBytesPerSec = SampleRate * 2,
                    cbSize = 0,
                };
                int err = Native.waveOutOpen(out IntPtr hwo, Native.WAVE_MAPPER, ref fmt,
                    evt.SafeWaitHandle.DangerousGetHandle(), IntPtr.Zero, Native.CALLBACK_EVENT);
                if (err != 0)
                {
                    // 没有声卡/设备被占用：2 秒后重试
                    evt.Dispose();
                    Thread.Sleep(2000);
                    continue;
                }

                var headers = new IntPtr[bufCount];
                var datas = new IntPtr[bufCount];
                for (int b = 0; b < bufCount; b++)
                {
                    datas[b] = Marshal.AllocHGlobal(bufSamples * 2);
                    headers[b] = Marshal.AllocHGlobal(hdrSize);
                    var h = new Native.WAVEHDR { lpData = datas[b], dwBufferLength = bufSamples * 2 };
                    Marshal.StructureToPtr(h, headers[b], false);
                    Native.waveOutPrepareHeader(hwo, headers[b], hdrSize);
                }

                bool failed = false;
                // 先把所有缓冲区填满送出
                for (int b = 0; b < bufCount && !failed; b++)
                    failed = !Submit(hwo, headers[b], datas[b], mix, pcm, hdrSize);

                while (_running && !failed)
                {
                    evt.WaitOne(100);
                    for (int b = 0; b < bufCount && !failed; b++)
                    {
                        var h = (Native.WAVEHDR)Marshal.PtrToStructure(headers[b], typeof(Native.WAVEHDR));
                        if ((h.dwFlags & Native.WHDR_DONE) != 0)
                            failed = !Submit(hwo, headers[b], datas[b], mix, pcm, hdrSize);
                    }
                }

                Native.waveOutReset(hwo);
                for (int b = 0; b < bufCount; b++)
                {
                    Native.waveOutUnprepareHeader(hwo, headers[b], hdrSize);
                    Marshal.FreeHGlobal(headers[b]);
                    Marshal.FreeHGlobal(datas[b]);
                }
                Native.waveOutClose(hwo);
                evt.Dispose();
                if (failed && _running) Thread.Sleep(500);   // 设备被拔掉等：稍后重开
            }
        }

        private bool Submit(IntPtr hwo, IntPtr hdr, IntPtr data, float[] mix, short[] pcm, int hdrSize)
        {
            Render(mix);
            for (int i = 0; i < mix.Length; i++)
            {
                float s = mix[i];
                if (s > 1f) s = 1f; else if (s < -1f) s = -1f;
                pcm[i] = (short)(s * 32767);
            }
            Marshal.Copy(pcm, 0, data, pcm.Length);
            // 清掉 DONE 标志后重新送出
            var h = (Native.WAVEHDR)Marshal.PtrToStructure(hdr, typeof(Native.WAVEHDR));
            h.dwFlags &= ~Native.WHDR_DONE;
            Marshal.StructureToPtr(h, hdr, false);
            return Native.waveOutWrite(hwo, hdr, hdrSize) == 0;
        }

        public void Dispose()
        {
            _running = false;
            _thread?.Join(1000);
        }
    }
}
