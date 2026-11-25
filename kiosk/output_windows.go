package kiosk

import (
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var propVariantClear = ole32.NewProc("PropVariantClear")

// Outputs lists the computer's active sound outputs, the default one marked
// Default, by the names the Sound settings show.
func Outputs() []Output {
	var outs []Output
	withEnumerator(func(enumerator *comObject) {
		def := ""
		const getDefaultEndpoint, enumEndpoints, render, console, active = 4, 3, 0, 0, 1
		var device *comObject
		if enumerator.call(getDefaultEndpoint, render, console, uintptr(unsafe.Pointer(&device))) == 0 && device != nil {
			def = deviceID(device)
			device.release()
		}
		var list *comObject
		if enumerator.call(enumEndpoints, render, active, uintptr(unsafe.Pointer(&list))) != 0 || list == nil {
			return
		}
		defer list.release()
		const getCount, item = 3, 4
		var n uint32
		if list.call(getCount, uintptr(unsafe.Pointer(&n))) != 0 {
			return
		}
		for i := range n {
			var dev *comObject
			if list.call(item, uintptr(i), uintptr(unsafe.Pointer(&dev))) != 0 || dev == nil {
				continue
			}
			id, name := deviceID(dev), friendlyName(dev)
			dev.release()
			if id != "" && name != "" {
				outs = append(outs, Output{ID: id, Name: name, Default: id == def})
			}
		}
	})
	return outs
}

// SetOutput makes the output id the default for every role, as choosing it
// in the Sound settings does; the piano's sound moves there with the
// system's. Windows has no public call for it: this is the policy
// interface the Sound settings themselves use, which every Windows since 7
// keeps.
func SetOutput(id string) bool {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if hr, _, _ := coInitializeEx.Call(0, 0); int32(hr) >= 0 {
		defer coUninitialize.Call()
	}
	clsid := windows.GUID{Data1: 0x870AF99C, Data2: 0x171D, Data3: 0x4F9E, Data4: [8]byte{0xAF, 0x0D, 0xE6, 0x3D, 0xF4, 0x0C, 0x2B, 0xC9}}
	iid := windows.GUID{Data1: 0xF8679F50, Data2: 0x850A, Data3: 0x41CF, Data4: [8]byte{0x9C, 0x72, 0x43, 0x0F, 0x29, 0x02, 0x90, 0xC8}}
	const clsctxAll = 0x17
	var policy *comObject
	if hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(&clsid)), 0, clsctxAll, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&policy))); hr != 0 || policy == nil {
		return false
	}
	defer policy.release()
	wid, err := windows.UTF16PtrFromString(id)
	if err != nil {
		return false
	}
	const setDefaultEndpoint = 13
	ok := true
	for role := range uintptr(3) { // console, multimedia, communications
		if policy.call(setDefaultEndpoint, uintptr(unsafe.Pointer(wid)), role) != 0 {
			ok = false
		}
	}
	runtime.KeepAlive(wid)
	return ok
}

// withEnumerator calls fn with Core Audio's device enumerator.
func withEnumerator(fn func(enumerator *comObject)) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if hr, _, _ := coInitializeEx.Call(0, 0); int32(hr) >= 0 {
		defer coUninitialize.Call()
	}
	clsid := windows.GUID{Data1: 0xBCDE0395, Data2: 0xE52F, Data3: 0x467C, Data4: [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iid := windows.GUID{Data1: 0xA95664D2, Data2: 0x9614, Data3: 0x4F35, Data4: [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	const clsctxAll = 0x17
	var enumerator *comObject
	if hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(&clsid)), 0, clsctxAll, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&enumerator))); hr != 0 || enumerator == nil {
		return
	}
	defer enumerator.release()
	fn(enumerator)
}

// deviceID is an IMMDevice's endpoint id.
func deviceID(dev *comObject) string {
	const getID = 5
	var p *uint16
	if dev.call(getID, uintptr(unsafe.Pointer(&p))) != 0 || p == nil {
		return ""
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(p))
	return windows.UTF16PtrToString(p)
}

// friendlyName is an IMMDevice's name, such as "Speakers (Realtek(R)
// Audio)", read from its property store.
func friendlyName(dev *comObject) string {
	const openPropertyStore, stgmRead = 4, 0
	var store *comObject
	if dev.call(openPropertyStore, stgmRead, uintptr(unsafe.Pointer(&store))) != 0 || store == nil {
		return ""
	}
	defer store.release()
	// PKEY_Device_FriendlyName
	key := struct {
		fmtid windows.GUID
		pid   uint32
	}{windows.GUID{Data1: 0xA45C254E, Data2: 0xDF1C, Data3: 0x4EFD, Data4: [8]byte{0x80, 0x20, 0x67, 0xD1, 0x46, 0xA8, 0x50, 0xE0}}, 14}
	// A PROPVARIANT: its type, three reserved words, then the value,
	// padded to its size on 64-bit Windows.
	var v struct {
		vt      uint16
		_, _, _ uint16
		val     *uint16
		_       uintptr
	}
	const getValue, vtLPWSTR = 5, 31
	if store.call(getValue, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&v))) != 0 {
		return ""
	}
	defer propVariantClear.Call(uintptr(unsafe.Pointer(&v)))
	if v.vt != vtLPWSTR || v.val == nil {
		return ""
	}
	return windows.UTF16PtrToString(v.val)
}
