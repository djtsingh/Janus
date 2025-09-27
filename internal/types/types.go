package types

type Fingerprint struct {
	CanvasHash    string   `json:"canvasHash"`
	WebglRenderer string   `json:"webglRenderer"`
	WebglVendor   string   `json:"webglVendor"`
	Language      string   `json:"language"`
	Platform      string   `json:"platform"`
	DoNotTrack    string   `json:"doNotTrack"`
	Plugins       []string `json:"plugins"`
	Screen        struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"screen"`
	Timezone  string `json:"timezone"`
	JSEnabled bool   `json:"jsEnabled"`
	IsMobile  bool   `json:"isMobile"`
}

type Verification struct {
	Proof string `json:"proof"`
	Nonce string `json:"nonce"`
}

type Challenge struct {
	Nonce      string `json:"nonce"`
	Iterations int    `json:"iterations"`
	Seed       string `json:"seed"`
}
