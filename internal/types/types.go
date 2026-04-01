package types

import (
	"sync"
	"time"
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
	BotScore  int    `json:"bot_score"`
}

type FingerprintStore struct {
	sync.RWMutex
	Data map[string]Fingerprint
}

type OffenderRecord struct {
	Attempts   int       `json:"attempts"`
	FirstSeen  time.Time `json:"firstSeen"`
	LastSeen   time.Time `json:"lastSeen"`
	TotalScore int       `json:"totalScore"`
	LastScore  int       `json:"lastScore"`
}

type Verification struct {
	Proof string `json:"proof"`
	Nonce string `json:"nonce"`
}

type Challenge struct {
	Nonce      string    `json:"nonce"`
	Iterations int       `json:"iterations"`
	Seed       string    `json:"seed"`
	Type       string    `json:"type"`
	Difficulty int       `json:"difficulty"`
	IssuedAt   time.Time `json:"issued_at"`
}

type BehavioralData struct {
	MouseMoves    int     `json:"mm"`
	MouseEntropy  float64 `json:"me"`
	KeyPresses    int     `json:"kp"`
	ScrollEvents  int     `json:"se"`
	TouchEvents   int     `json:"te"`
	InteractionMs int64   `json:"im"`
}
