package piano

import "strconv"

// macNames names macOS virtual key codes (kVK_*) by keyRows' names.
var macNames = map[int]string{
	// The bottom row. A Mac's fn sits where a PC's Insert does.
	59: "ctrl", 58: "alt", 55: "meta", 49: "space", 54: "rmeta", 61: "ralt", 62: "rctrl",
	123: "left", 125: "down", 124: "right", 82: "kp0", 65: "kp.",
	56: "shift", 6: "z", 7: "x", 8: "c", 9: "v", 11: "b", 45: "n", 46: "m", 43: ",", 47: ".", 44: "/", 60: "rshift",
	126: "up", 83: "kp1", 84: "kp2", 85: "kp3", 76: "kpenter",
	57: "capslock", 0: "a", 1: "s", 2: "d", 3: "f", 5: "g", 4: "h", 38: "j", 40: "k", 37: "l", 41: ";", 39: "'", 36: "enter",
	86: "kp4", 87: "kp5", 88: "kp6", 69: "kp+mac",
	48: "tab", 12: "q", 13: "w", 14: "e", 15: "r", 17: "t", 16: "y", 32: "u", 34: "i", 31: "o", 35: "p", 33: "[", 30: "]", 42: "\\",
	117: "delete", 119: "end", 121: "pagedown", 89: "kp7", 91: "kp8", 92: "kp9", 78: "kp+",
	50: "`", 18: "1", 19: "2", 20: "3", 21: "4", 23: "5", 22: "6", 26: "7", 28: "8", 25: "9", 29: "0", 27: "-", 24: "=", 51: "backspace",
	// A Mac keypad's top row is Clear = / *, where a PC's is Num Lock / * -.
	63: "insert", 114: "insert", 115: "home", 116: "pageup", 71: "numlock", 81: "kp/", 75: "kp*", 67: "kp-",
	53: "escape", 122: "f1", 120: "f2", 99: "f3", 118: "f4", 96: "f5", 97: "f6", 98: "f7", 100: "f8",
	101: "f9", 109: "f10", 103: "f11", 111: "f12", 105: "print", 107: "scrolllock", 113: "pause",
}

// windowsKey is a scan code (set 1) with its E0 prefix, which arrows, the
// right-hand modifiers and most navigation keys have: a key by where it is,
// whatever the layout.
type windowsKey struct {
	scan     int
	extended bool
}

// windowsNames names windowsKeys by keyRows' names.
var windowsNames = func() map[windowsKey]string {
	plain := map[int]string{
		0x1D: "ctrl", 0x38: "alt", 0x39: "space", 0x52: "kp0", 0x53: "kp.",
		0x2A: "shift", 0x2C: "z", 0x2D: "x", 0x2E: "c", 0x2F: "v", 0x30: "b", 0x31: "n", 0x32: "m",
		0x33: ",", 0x34: ".", 0x35: "/", 0x36: "rshift", 0x4F: "kp1", 0x50: "kp2", 0x51: "kp3",
		0x3A: "capslock", 0x1E: "a", 0x1F: "s", 0x20: "d", 0x21: "f", 0x22: "g", 0x23: "h", 0x24: "j",
		0x25: "k", 0x26: "l", 0x27: ";", 0x28: "'", 0x1C: "enter", 0x4B: "kp4", 0x4C: "kp5", 0x4D: "kp6",
		0x0F: "tab", 0x10: "q", 0x11: "w", 0x12: "e", 0x13: "r", 0x14: "t", 0x15: "y", 0x16: "u",
		0x17: "i", 0x18: "o", 0x19: "p", 0x1A: "[", 0x1B: "]", 0x2B: "\\", 0x47: "kp7", 0x48: "kp8", 0x49: "kp9", 0x4E: "kp+",
		0x29: "`", 0x02: "1", 0x03: "2", 0x04: "3", 0x05: "4", 0x06: "5", 0x07: "6", 0x08: "7",
		0x09: "8", 0x0A: "9", 0x0B: "0", 0x0C: "-", 0x0D: "=", 0x0E: "backspace", 0x37: "kp*", 0x4A: "kp-",
		0x01: "escape", 0x3B: "f1", 0x3C: "f2", 0x3D: "f3", 0x3E: "f4", 0x3F: "f5", 0x40: "f6", 0x41: "f7",
		0x42: "f8", 0x43: "f9", 0x44: "f10", 0x57: "f11", 0x58: "f12", 0x46: "scrolllock",
		// A low-level hook reports Pause as 45 without E0 and Num Lock with it.
		0x45: "pause",
	}
	extended := map[int]string{
		0x5B: "meta", 0x38: "ralt", 0x5C: "rmeta", 0x5D: "menu", 0x1D: "rctrl",
		0x4B: "left", 0x50: "down", 0x4D: "right", 0x48: "up", 0x1C: "kpenter",
		0x53: "delete", 0x4F: "end", 0x51: "pagedown",
		0x52: "insert", 0x47: "home", 0x49: "pageup", 0x45: "numlock", 0x35: "kp/",
		0x37: "print",
	}
	names := map[windowsKey]string{}
	for scan, name := range plain {
		names[windowsKey{scan, false}] = name
	}
	for scan, name := range extended {
		names[windowsKey{scan, true}] = name
	}
	return names
}()

// MacKeyName names the key with macOS virtual key code code for NoteForKey.
func MacKeyName(code int) string {
	if name, ok := macNames[code]; ok {
		return name
	}
	return "mac:" + strconv.Itoa(code)
}

// MacMediaKeyName names a media, brightness or volume key by its NX key type.
func MacMediaKeyName(keyType int) string { return "media:" + strconv.Itoa(keyType) }

// WindowsKeyName names the key with scan code scan, E0-prefixed when extended,
// for NoteForKey.
func WindowsKeyName(scan int, extended bool) string {
	if name, ok := windowsNames[windowsKey{scan, extended}]; ok {
		return name
	}
	if extended {
		return "win:e0:" + strconv.Itoa(scan)
	}
	return "win:" + strconv.Itoa(scan)
}
