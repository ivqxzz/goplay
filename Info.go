package main

import "howett.net/plist"

var deviceInfo = map[string]interface{}{
	"deviceID":          "AA:BB:CC:DD:EE:FF",
	"macAddress":        "AA:BB:CC:DD:EE:FF",
	"features":          uint64(0x1E5A7FFFF6),
	"statusFlags":       uint64(0x44),
	"keepAliveLowPower": 1,
	"model":             "AppleTV3,2",
	"name":              "GoPlay",
	"protocolVersion":   "1.1",
	"sourceVersion":     "220.68",
	"pi":                "b08f5a79-db29-4384-b456-a4784d9e6055",
	"pk":                make([]byte, 32),
	"vv":                2,
	"displays": []map[string]interface{}{
		{
			"widthPixels":  1920,
			"heightPixels": 1080,
			"width":        1920,
			"height":       1080,
			"refreshRate":  60,
			"maxFPS":       60,
			"overscanned":  false,
			"rotation":     false,
			"features":     14,
			"uuid":         "e0ff8a27-6738-3d56-8a16-cc53aacee925",
		},
	},
	"audioFormats": []map[string]interface{}{},
}

func infoPlist() ([]byte, error) {
	return plist.Marshal(deviceInfo, plist.BinaryFormat)
}
