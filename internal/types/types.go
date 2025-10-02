package types

import (
	"sync"
)

// This struct defines the data collected from the client's browser.
type Fingerprint struct {
	ClientIP      string `json:"client_ip"`
	Plugins       string `json:"plugins"`
	HardwareCon   int    `json:"hardware_concurrency"`
	Webdriver     bool   `json:"webdriver"`
	ChromeExists  bool   `json:"chrome_exists"`
	CanvasHash    string `json:"canvas_hash"`
	ScreenRes     string `json:"screen_resolution"`
	ColorDepth    int    `json:"color_depth"`
	Fonts         string `json:"fonts"`
	WebGLRenderer string `json:"webgl_renderer"`
	JA3           string `json:"ja3"`
	Screen        struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"screen"`
	Timezone  string `json:"timezone"`
	JSEnabled bool   `json:"jsEnabled"`
	IsMobile  bool   `json:"isMobile"`
}

// This struct holds the fingerprint data on the server.
type FingerprintStore struct {
	sync.RWMutex
	// The "types." prefix is removed because we are already inside the "types" package.
	Data map[string]Fingerprint
}

// This struct is used for decoding the verification request from the client.
type Verification struct {
	Proof string `json:"proof"`
	Nonce string `json:"nonce"`
}

// This struct defines the components of a proof-of-work challenge.
type Challenge struct {
	Nonce      string
	Iterations int
	Seed       string
}
