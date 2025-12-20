package types

import (
	"sync"
)

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

type FingerprintStore struct {
	sync.RWMutex
	Data map[string]Fingerprint
}

type Verification struct {
	Proof string `json:"proof"`
	Nonce string `json:"nonce"`
}

type Challenge struct {
	Nonce      string
	Iterations int
	Seed       string
	Type       string
	Difficulty int
}
