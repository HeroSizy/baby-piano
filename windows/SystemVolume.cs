using System;
using System.Runtime.InteropServices;

namespace BabyPiano
{
    /// <summary>
    /// 读写 Windows 主音量（Core Audio IAudioEndpointVolume）。
    /// 弹琴时把系统音量压在上限以下；中途换了蓝牙音箱/耳机也会被看门狗压回去。
    /// </summary>
    internal static class SystemVolume
    {
        public const float MaxSystemVolume = 0.5f;

        public static float? Get()
        {
            try
            {
                var ep = GetEndpoint();
                if (ep == null) return null;
                try
                {
                    Marshal.ThrowExceptionForHR(ep.GetMasterVolumeLevelScalar(out float v));
                    return v;
                }
                finally { Marshal.ReleaseComObject(ep); }
            }
            catch { return null; }
        }

        public static bool Set(float value)
        {
            try
            {
                var ep = GetEndpoint();
                if (ep == null) return false;
                try
                {
                    var ctx = Guid.Empty;
                    return ep.SetMasterVolumeLevelScalar(value, ref ctx) == 0;
                }
                finally { Marshal.ReleaseComObject(ep); }
            }
            catch { return false; }
        }

        public static void EnforceCap()
        {
            var v = Get();
            if (v.HasValue && v.Value > MaxSystemVolume + 0.005f) Set(MaxSystemVolume);
        }

        private static IAudioEndpointVolume GetEndpoint()
        {
            var enumerator = (IMMDeviceEnumerator)new MMDeviceEnumeratorComObject();
            try
            {
                // eRender = 0, eMultimedia = 1
                if (enumerator.GetDefaultAudioEndpoint(0, 1, out IMMDevice device) != 0 || device == null)
                    return null;
                try
                {
                    var iid = typeof(IAudioEndpointVolume).GUID;
                    const int CLSCTX_ALL = 0x17;
                    if (device.Activate(ref iid, CLSCTX_ALL, IntPtr.Zero, out object o) != 0) return null;
                    return (IAudioEndpointVolume)o;
                }
                finally { Marshal.ReleaseComObject(device); }
            }
            finally { Marshal.ReleaseComObject(enumerator); }
        }

        [ComImport, Guid("BCDE0395-E52F-467C-8E3D-C4579291692E")]
        private class MMDeviceEnumeratorComObject { }

        [ComImport, Guid("A95664D2-9614-4F35-A746-DE8DB63617E6"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
        private interface IMMDeviceEnumerator
        {
            [PreserveSig] int EnumAudioEndpoints(int dataFlow, int stateMask, out IntPtr devices);
            [PreserveSig] int GetDefaultAudioEndpoint(int dataFlow, int role, out IMMDevice endpoint);
            [PreserveSig] int GetDevice([MarshalAs(UnmanagedType.LPWStr)] string id, out IMMDevice device);
            [PreserveSig] int RegisterEndpointNotificationCallback(IntPtr client);
            [PreserveSig] int UnregisterEndpointNotificationCallback(IntPtr client);
        }

        [ComImport, Guid("D666063F-1587-4E43-81F1-B948E807363F"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
        private interface IMMDevice
        {
            [PreserveSig] int Activate(ref Guid iid, int clsCtx, IntPtr activationParams,
                [MarshalAs(UnmanagedType.IUnknown)] out object iface);
            [PreserveSig] int OpenPropertyStore(int access, out IntPtr properties);
            [PreserveSig] int GetId([MarshalAs(UnmanagedType.LPWStr)] out string id);
            [PreserveSig] int GetState(out int state);
        }

        [ComImport, Guid("5CDF2C82-841E-4546-9722-0CF74078229A"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
        private interface IAudioEndpointVolume
        {
            [PreserveSig] int RegisterControlChangeNotify(IntPtr notify);
            [PreserveSig] int UnregisterControlChangeNotify(IntPtr notify);
            [PreserveSig] int GetChannelCount(out uint channelCount);
            [PreserveSig] int SetMasterVolumeLevel(float levelDB, ref Guid eventContext);
            [PreserveSig] int SetMasterVolumeLevelScalar(float level, ref Guid eventContext);
            [PreserveSig] int GetMasterVolumeLevel(out float levelDB);
            [PreserveSig] int GetMasterVolumeLevelScalar(out float level);
            [PreserveSig] int SetChannelVolumeLevel(uint channel, float levelDB, ref Guid eventContext);
            [PreserveSig] int SetChannelVolumeLevelScalar(uint channel, float level, ref Guid eventContext);
            [PreserveSig] int GetChannelVolumeLevel(uint channel, out float levelDB);
            [PreserveSig] int GetChannelVolumeLevelScalar(uint channel, out float level);
            [PreserveSig] int SetMute([MarshalAs(UnmanagedType.Bool)] bool mute, ref Guid eventContext);
            [PreserveSig] int GetMute([MarshalAs(UnmanagedType.Bool)] out bool mute);
            [PreserveSig] int GetVolumeStepInfo(out uint step, out uint stepCount);
            [PreserveSig] int VolumeStepUp(ref Guid eventContext);
            [PreserveSig] int VolumeStepDown(ref Guid eventContext);
            [PreserveSig] int QueryHardwareSupport(out uint hardwareSupportMask);
            [PreserveSig] int GetVolumeRange(out float volumeMinDB, out float volumeMaxDB, out float volumeIncrementDB);
        }
    }
}
