package handlers

import (
	"encoding/json"
	"janus/internal/config"
	"janus/internal/store"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// --- Data Structures for JSON Payloads ---
type MousePoint struct {
	X int   `json:"x"`
	Y int   `json:"y"`
	T int64 `json:"t"`
}
type ActivityPackage struct {
	Activity       string       `json:"activity"`
	MouseSignature []MousePoint `json:"mouseSignature,omitempty"`
}
type AccelerometerReading struct {
	X *float64 `json:"x"`
	Y *float64 `json:"y"`
	Z *float64 `json:"z"`
}
type AccelerometerData struct {
	Reading1 *AccelerometerReading `json:"reading1"`
	Reading2 *AccelerometerReading `json:"reading2"`
}
type BatteryData struct {
	Charging bool    `json:"charging"`
	Level    float64 `json:"level"`
}
type ForensicsPackage struct {
	DeviceType        string             `json:"deviceType"`
	ScreenWidth       int                `json:"screenWidth"`
	ScreenHeight      int                `json:"screenHeight"`
	CanvasFingerprint string             `json:"canvasFingerprint"`
	AccelerometerData *AccelerometerData `json:"accelerometerData"`
	BatteryData       *BatteryData       `json:"batteryData"`
	MouseSignature    []MousePoint       `json:"mouseSignature"`
}

// --- Handler Setup ---
type AppHandlers struct {
	Cfg *config.Config
	St  *store.Store
}

func New(cfg *config.Config, st *store.Store) *AppHandlers {
	return &AppHandlers{Cfg: cfg, St: st}
}

// --- Endpoint Handlers ---

func (h *AppHandlers) VerifyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var pkg ForensicsPackage
	if err := json.NewDecoder(r.Body).Decode(&pkg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Println("--- JANUS VERIFICATION ---")
	log.Printf("Received Forensics Package from a '%s' device.", pkg.DeviceType)

	isVerified := false
	switch pkg.DeviceType {
	case "mobile", "tablet":
		isVerified = h.handleMobileVerification(pkg)
	case "desktop":
		isVerified = h.handleDesktopVerification(pkg)
	default:
		log.Println("DECISION: BLOCKED (Reason: Unknown device type)")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if !isVerified {
		json.NewEncoder(w).Encode(map[string]string{"status": "failed"})
		return
	}

	token := uuid.New().String()
	session := &store.Session{
		VerifiedAt:              time.Now(),
		LastSeen:                time.Now(),
		HasScrolled:             false,
		HasNaturalMouseMovement: false,
		PagesViewed:             1,
		NavigationPath:          []string{r.URL.Path},
	}
	// Save the new session to Redis with the configured timeout.
	err := h.St.SetSession(token, session, time.Duration(h.Cfg.SessionTimeoutSec)*time.Second)
	if err != nil {
		log.Printf("ERROR: Failed to set session in Redis: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("Verification PASSED. Granting session token via cookie: %s", token)
	http.SetCookie(w, &http.Cookie{
		Name:     "janus-session-token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	json.NewEncoder(w).Encode(map[string]string{"status": "verified"})
}

func (h *AppHandlers) TelemetryHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("janus-session-token")
	if err != nil {
		http.Error(w, "Missing session token", http.StatusForbidden)
		return
	}
	token := cookie.Value

	var pkg ActivityPackage
	if err := json.NewDecoder(r.Body).Decode(&pkg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	session, exists := h.St.GetSession(token)
	if !exists {
		http.Error(w, "Invalid session token", http.StatusForbidden)
		return
	}

	session.LastSeen = time.Now()
	switch pkg.Activity {
	case "scroll":
		session.HasScrolled = true
		log.Printf("CONTINUOUS MONITORING: 'scroll' activity for token %s", token)
	case "mousemove":
		if !isLinear(pkg.MouseSignature) {
			session.HasNaturalMouseMovement = true
			log.Printf("CONTINUOUS MONITORING: Natural mouse movement detected for token %s", token)
		}
	}
	// Save the updated session back to Redis
	h.St.SetSession(token, session, time.Duration(h.Cfg.SessionTimeoutSec)*time.Second)
	w.WriteHeader(http.StatusOK)
}

// --- Verification Logic Helpers ---

func (h *AppHandlers) handleDesktopVerification(pkg ForensicsPackage) bool {
	log.Println("Routing to DESKTOP verification path.")
	// Initial check is now more lenient: we only check for a valid canvas fingerprint.
	// We no longer block for lack of initial mouse movement.
	if pkg.CanvasFingerprint == "" || pkg.CanvasFingerprint == "CanvasError" {
		log.Println("DECISION: BLOCKED (Reason: Canvas fingerprint failed)")
		return false
	}
	log.Println("DECISION: ALLOWED (Canvas check passed, continuous monitoring will begin)")
	return true
}

func (h *AppHandlers) handleMobileVerification(pkg ForensicsPackage) bool {
	log.Println("Routing to MOBILE verification path.")
	if pkg.AccelerometerData == nil || pkg.AccelerometerData.Reading1 == nil || pkg.AccelerometerData.Reading2 == nil {
		log.Println("DECISION: BLOCKED (Reason: Accelerometer data not available)")
		return false
	}
	r1, r2 := pkg.AccelerometerData.Reading1, pkg.AccelerometerData.Reading2
	if r1.X != nil && r1.X == r2.X && r1.Y == r2.Y && r1.Z == r2.Z {
		log.Println("DECISION: BLOCKED (Reason: Accelerometer readings are static - likely an emulator)")
		return false
	}
	if pkg.BatteryData == nil {
		log.Println("DECISION: BLOCKED (Reason: Battery data not available)")
		return false
	}
	if pkg.BatteryData.Level == 1.0 && pkg.BatteryData.Charging {
		log.Println("DECISION: BLOCKED (Reason: Battery is 100% and charging - likely an emulator)")
		return false
	}
	log.Println("DECISION: ALLOWED (Mobile sensor checks passed)")
	return true
}

func isLinear(points []MousePoint) bool {
	if len(points) < 3 {
		return true // Not enough data to analyze, assume suspicious
	}
	dx1 := float64(points[1].X - points[0].X)
	dy1 := float64(points[1].Y - points[0].Y)
	for i := 2; i < len(points); i++ {
		dx2 := float64(points[i].X - points[i-1].X)
		dy2 := float64(points[i].Y - points[i-1].Y)
		if dx1*dy2 != dx2*dy1 {
			return false // Slopes differ, path is not linear
		}
	}
	return true // Path is linear
}
