//go:build darwin && !ios

package kiosk

import (
	"strconv"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Core Audio's property selectors and scopes, as four-character codes.
const (
	selDevices       = 0x64657623 // 'dev#' kAudioHardwarePropertyDevices
	selDefaultOutput = 0x644F7574 // 'dOut' kAudioHardwarePropertyDefaultOutputDevice
	selStreams       = 0x73746D23 // 'stm#' kAudioDevicePropertyStreams
	selName          = 0x6C6E616D // 'lnam' kAudioObjectPropertyName
	selHidden        = 0x6869646E // 'hidn' kAudioDevicePropertyIsHidden
	scopeGlobal      = 0x676C6F62 // 'glob'
	scopeOutput      = 0x6F757470 // 'outp'
	systemObject     = 1          // kAudioObjectSystemObject
)

var cfStringGetCString func(str uintptr, buf *byte, size int, encoding uint32) bool

var loadStrings = sync.OnceFunc(func() {
	load()
	purego.RegisterLibFunc(&cfStringGetCString, coreFoundation, "CFStringGetCString")
})

// Outputs lists the Mac's sound outputs, the one it plays through marked
// Default: devices with output streams that the system does not hide, such
// as aggregate devices' parts.
func Outputs() []Output {
	loadStrings()
	var ids [128]uint32
	size := uint32(unsafe.Sizeof(ids))
	addr := audioPropertyAddress{selDevices, scopeGlobal, 0}
	if audioObjectGetPropertyData(systemObject, &addr, 0, 0, &size, unsafe.Pointer(&ids)) != 0 {
		return nil
	}
	def := defaultOutputDevice()
	var outs []Output
	for _, dev := range ids[:size/4] {
		if !hasOutput(dev) || hidden(dev) {
			continue
		}
		name := deviceName(dev)
		if name == "" {
			continue
		}
		outs = append(outs, Output{ID: strconv.FormatUint(uint64(dev), 10), Name: name, Default: dev == def})
	}
	return outs
}

// SetOutput makes the output id the Mac's default, as choosing it in
// Control Center does; the piano's sound moves there with the system's.
func SetOutput(id string) bool {
	loadStrings()
	n, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return false
	}
	dev := uint32(n)
	addr := audioPropertyAddress{selDefaultOutput, scopeGlobal, 0}
	return audioObjectSetPropertyData(systemObject, &addr, 0, 0, 4, unsafe.Pointer(&dev)) == 0
}

func hasOutput(dev uint32) bool {
	var streams [16]uint32
	size := uint32(unsafe.Sizeof(streams))
	addr := audioPropertyAddress{selStreams, scopeOutput, 0}
	return audioObjectGetPropertyData(dev, &addr, 0, 0, &size, unsafe.Pointer(&streams)) == 0 && size > 0
}

func hidden(dev uint32) bool {
	addr := audioPropertyAddress{selHidden, scopeGlobal, 0}
	if !audioObjectHasProperty(dev, &addr) {
		return false
	}
	var v uint32
	size := uint32(4)
	return audioObjectGetPropertyData(dev, &addr, 0, 0, &size, unsafe.Pointer(&v)) == 0 && v != 0
}

// deviceName is the device's name, as the Sound settings show it.
func deviceName(dev uint32) string {
	addr := audioPropertyAddress{selName, scopeGlobal, 0}
	var str uintptr
	size := uint32(unsafe.Sizeof(str))
	if audioObjectGetPropertyData(dev, &addr, 0, 0, &size, unsafe.Pointer(&str)) != 0 || str == 0 {
		return ""
	}
	defer cfRelease(str)
	var buf [256]byte
	const utf8 = 0x08000100 // kCFStringEncodingUTF8
	if !cfStringGetCString(str, &buf[0], len(buf), utf8) {
		return ""
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return ""
}
