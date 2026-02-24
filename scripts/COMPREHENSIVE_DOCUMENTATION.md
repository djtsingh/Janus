# Janus - Comprehensive Project Documentation

## Table of Contents

1. [Overview](#1-overview)
2. [Core Philosophy & Problem Statement](#2-core-philosophy--problem-statement)
3. [Architecture](#3-architecture)
4. [Technology Stack](#4-technology-stack)
5. [Project Structure](#5-project-structure)
6. [Core Components Deep Dive](#6-core-components-deep-dive)
7. [Workflow & Data Flow](#7-workflow--data-flow)
8. [Configuration](#8-configuration)
9. [Frontend Components](#9-frontend-components)
10. [Backend Components](#10-backend-components)
11. [Security Mechanisms](#11-security-mechanisms)
12. [API Reference](#12-api-reference)
13. [Docker & Deployment](#13-docker--deployment)
14. [Testing](#14-testing)
15. [Roadmap & Future Enhancements](#15-roadmap--future-enhancements)
16. [Glossary](#16-glossary)

---

## 1. Overview

### What is Janus?

**Janus** is a lightweight Go-based middleware designed to protect web applications from automated bots while maintaining a smooth, frictionless experience for legitimate human users. Named after the Roman god of doorways and transitions (who had two faces), Janus serves as the intelligent gatekeeper between your users and your protected web application.

### Unique Selling Points (USP)

- **Less Friction than CAPTCHAs**: Challenges are small, transparent, and often invisible to real users
- **Multi-Signal Detection**: Uses a weighted scoring system combining multiple signals rather than a single noisy heuristic
- **Fully Configurable**: All difficulty levels, weights, and whitelists are tunable via `config.yaml`
- **Dual-Path Verification**: Different verification strategies for desktop (Proof-of-Render) vs mobile (sensor-based)
- **Continuous Monitoring**: "Zombie detection" catches bots that pass initial verification but behave non-humanly

### How It Differs from Traditional CAPTCHAs

| Traditional CAPTCHA | Janus |
|---------------------|-------|
| Visible interruption | Often invisible to legitimate users |
| Single challenge | Multi-layered scoring system |
| Binary pass/fail | Progressive challenge escalation |
| No behavioral analysis | Continuous session monitoring |
| High user friction | Minimal friction for humans |

---

## 2. Core Philosophy & Problem Statement

### The Bot Problem

Modern web applications face several threats from automated traffic:
- **Web Scrapers**: Extract content and data
- **Credential Stuffers**: Attempt brute-force logins
- **DDoS Bots**: Overwhelm servers with requests
- **Spam Bots**: Submit unwanted content
- **Price Scrapers**: Monitor pricing for competitive advantage

### Why Traditional Solutions Fall Short

1. **CAPTCHAs are annoying**: They create friction for legitimate users
2. **Headless browsers can solve them**: Modern bots using Puppeteer/Playwright can solve many challenges
3. **Single-point failures**: One-and-done verification allows bots to reuse tokens
4. **No behavior analysis**: They don't monitor post-verification behavior

### Janus's Approach: Multi-Layered Defense

```
┌─────────────────────────────────────────────────────────────┐
│                    LAYERED DEFENSE SYSTEM                  │
├─────────────────────────────────────────────────────────────┤
│  Layer 1: Rate Limiting (IP-based)                         │
│  Layer 2: IP/Geo Reputation Check                          │
│  Layer 3: TLS/JA3 Fingerprinting                           │
│  Layer 4: Browser Fingerprinting (canvas, WebGL, etc.)     │
│  Layer 5: Proof-of-Work / Proof-of-Render Challenge        │
│  Layer 6: Continuous Behavioral Monitoring (Zombie Check)  │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Architecture

### High-Level System Architecture

```
                               ┌──────────────────┐
                               │   Clients        │
                               │(Desktop / Mobile)│
                               └──────┬───────────┘
                                      │ HTTPS
                                      ▼
                         ┌────────────────────────────┐
                         │       Edge / Load Balancer │
                         │   (optional: Nginx/Caddy)  │
                         └──────┬─────────────────────┘
                                │
                                ▼
  ┌─────────────────────────────────────────────────────────────────────────────┐
  │                                 Janus Proxy                                 │
  │ ┌──────────────────┐   ┌──────────────────┐   ┌─────────────────────────┐   │
  │ │Request Filtering │   │Injector (JS)     │   │Decision Engine / Scorer │   │
  │ │- Rate limit      │   │- inject sensor.js│   │- rules + ML hooks       │   │
  │ │- IP allow/deny   │   │  & proof_render  │   │- session scoring + bans │   │
  │ └──────────────────┘   └──────────────────┘   └─────────┬───────────────┘   │
  │                                                           │                  │
  │                                ┌─────────────────────────────┐               │
  │                                │  Session Store (Redis)      │               │
  │                                │  - score, verified, bans    │               │
  │                                │  - rate-limiter token state │               │
  │                                └──────────────────────────────┘               │
  └─────────────────────────────────────────────────────────────────────────────┘
                │                      
                │ proxy allowed/deny    
                ▼                      
         ┌────────────┐         
         │  Origin    │         
         │  Web App   │         
         └────────────┘         
```

### Component Descriptions

| Component | Purpose |
|-----------|---------|
| **Request Filtering** | First-pass checks: rate limiting, IP blacklists, geo-blocking |
| **JS Injector** | Serves challenge page with sensor.js for fingerprinting |
| **Decision Engine** | Analyzes signals, computes suspicion scores, makes allow/challenge/block decisions |
| **Session Store (Redis)** | Stores session state, fingerprints, challenge data, rate limits |
| **Origin Web App** | The protected backend application |

---

## 4. Technology Stack

### Backend (Go)

| Technology | Purpose |
|------------|---------|
| **Go 1.25+** | Core language - single binary, low latency, high concurrency |
| **chi/v5** | HTTP router for REST endpoints |
| **go-redis/redis/v8** | Redis client for session storage and rate limiting |
| **golang-jwt/jwt/v5** | JWT token generation and validation |
| **oschwald/geoip2-golang/v2** | GeoIP lookups using MaxMind database |
| **gopkg.in/yaml.v3** | YAML configuration parsing |
| **google/uuid** | UUID generation for nonces |

### Frontend (JavaScript)

| Technology | Purpose |
|------------|---------|
| **Vanilla JavaScript** | Client-side fingerprinting and challenge solving |
| **Web Crypto API** | SHA-256 hashing for Proof-of-Work |
| **Canvas API** | Browser fingerprinting |
| **WebGL API** | GPU/driver fingerprinting |

### Infrastructure

| Technology | Purpose |
|------------|---------|
| **Redis** | Session store, rate limiting, nonce validation |
| **Docker** | Containerization |
| **Docker Compose** | Local development orchestration |
| **GeoLite2-City.mmdb** | MaxMind GeoIP database for geographic checks |

---

## 5. Project Structure

```
janus/
├── cmd/
│   └── janus/
│       └── main.go              # Application entrypoint
├── internal/
│   ├── challenge/
│   │   └── challenge.go         # Challenge generation & verification
│   ├── config/
│   │   └── config.go            # Configuration loading & defaults
│   ├── handlers/
│   │   └── handlers.go          # HTTP handlers for fingerprint endpoint
│   ├── middleware/
│   │   └── janus.go             # Core middleware (514 lines of logic)
│   ├── ratelimit/
│   │   └── ratelimit.go         # Local rate limiting (fallback)
│   ├── store/
│   │   └── store.go             # Redis session/nonce store
│   └── types/
│       └── types.go             # Shared types (Fingerprint, Challenge)
├── assets/
│   ├── challenge.html           # Challenge page HTML
│   └── sensor.js                # Client-side fingerprinting script
├── example/
│   ├── package.json             # Example Node.js origin server
│   └── test.html                # Test page for development
├── scripts/
│   ├── run_flow.sh              # Bash test script
│   └── test_janus_flow.sh       # Integration test script
├── Js-docs/
│   ├── architecture.md          # Architecture documentation
│   ├── development.md           # Developer guide
│   ├── README.md                # Docs overview
│   ├── setup.md                 # Setup instructions
│   └── testing.md               # Testing guide
├── metadata/
│   ├── JANUS                    # Detailed design document
│   ├── MVP_0.1v                 # MVP specification
│   ├── system_architecture      # System architecture details
│   └── techstack                # Technology stack info
├── config.yaml                  # Active configuration
├── config.example.yaml          # Example configuration
├── docker-compose.yml           # Docker Compose setup
├── Dockerfile                   # Docker build file
├── entrypoint.sh                # Container entrypoint script
├── go.mod                       # Go module definition
├── server.js                    # Simple origin server for testing
├── GeoLite2-City.mmdb           # GeoIP database (not in repo)
├── cert.pem & key.pem           # TLS certificates (generated)
├── README.md                    # Project README
├── LICENSE                      # License file
├── CONTRIBUTING.md              # Contribution guidelines
└── CODE_OF_CONDUCT.md           # Code of conduct
```

---

## 6. Core Components Deep Dive

### 6.1 Main Entry Point (`cmd/janus/main.go`)

The entry point sets up the HTTP server with TLS support:

```go
func main() {
    r := chi.NewRouter()
    r.Use(middleware.JanusMiddleware)  // Apply Janus protection to all routes
    
    // Protected root route
    r.Get("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Welcome to your protected site!"))
    })
    
    // Serve sensor.js asset
    r.Get("/sensor.js", func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, "assets/sensor.js")
    })
    
    // TLS configuration
    httpsServer := &http.Server{
        Addr:    ":8080",
        Handler: r,
        TLSConfig: &tls.Config{
            Certificates: []tls.Certificate{cert},
            MinVersion:   tls.VersionTLS12,
        },
    }
    
    // HTTP redirect server (8081 → 8080 HTTPS)
    httpServer := &http.Server{
        Addr: ":8081",
        Handler: http.HandlerFunc(redirectToHTTPS),
    }
}
```

**Key Points:**
- Uses chi router for HTTP routing
- JanusMiddleware is applied globally to protect all routes
- Runs HTTPS on port 8080 with TLS 1.2+
- HTTP on port 8081 redirects to HTTPS

### 6.2 Core Middleware (`internal/middleware/janus.go`)

This is the heart of Janus - a 514-line middleware that handles:

#### Initialization

```go
var (
    fingerprintStore = &types.FingerprintStore{Data: make(map[string]types.Fingerprint)}
    challengeStore   = &ChallengeStore{...}
    loadedConfig     *config.JanusConfig
    jwtSecret        = []byte("your-secure-random-secret-key-32bytes")
    geoDB            *geoip2.Reader
)

func init() {
    // Initialize Janus API router
    janusRouter = chi.NewRouter()
    janusRouter.Post("/janus/fingerprint", handlers.HandleFingerprint(fingerprintStore))
    janusRouter.Get("/janus/challenge", handleChallenge)
    janusRouter.Post("/janus/verify", handleVerify)
    
    // Load GeoIP database
    geoDB, err = geoip2.Open("GeoLite2-City.mmdb")
    
    // Start challenge cleanup goroutine
    go func() {
        for {
            time.Sleep(1 * time.Minute)
            // Cleanup expired challenges
        }
    }()
}
```

#### Request Processing Flow

```go
func JanusMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        clientIP := getClientIP(r)
        
        // 1. Handle Janus API endpoints
        if strings.HasPrefix(r.URL.Path, "/janus/") {
            janusRouter.ServeHTTP(w, r)
            return
        }
        
        // 2. Serve sensor.js directly
        if r.URL.Path == "/sensor.js" {
            http.ServeFile(w, r, "assets/sensor.js")
            return
        }
        
        // 3. Check rate limits
        if limited, _ := redisStore.IsRateLimited(clientIP, rateLimit); limited {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        
        // 4. Check if user is already verified
        if isVerified(r) {
            next.ServeHTTP(w, r)  // Allow through
            return
        }
        
        // 5. Calculate suspicion score
        suspicious, score := isSuspicious(r, loadedConfig)
        
        // 6. Issue challenge
        issueChallenge(w, r)
    })
}
```

#### Suspicion Scoring System

The `isSuspicious()` function calculates a weighted score based on multiple signals:

| Signal | Weight (Default) | Trigger |
|--------|------------------|---------|
| `blacklisted_ip` | 100 | IP in blacklist |
| `banned_geo` | 80-100 | Geographic location banned |
| `tls_mismatch` | 30 | JA3 fingerprint doesn't match known browsers |
| `no_user_agent` | 40 | Missing or suspicious User-Agent |
| `headless_browser` | 50 | Detected headless browser indicators |
| `missing_headers` | 20 | Missing Accept header |
| `header_order_mismatch` | 20 | Missing expected browser headers |
| `no_fingerprint` | 30 | No fingerprint submitted |

```go
func isSuspicious(r *http.Request, cfg *config.JanusConfig) (bool, int) {
    score := 0
    
    // Check blacklisted IPs
    for _, blacklistedIP := range cfg.BlacklistedIPs {
        if clientIP == blacklistedIP {
            score += cfg.SuspicionWeights["blacklisted_ip"]
        }
    }
    
    // Check geo-location
    if geoDB != nil {
        record, _ := geoDB.City(ipAddr)
        for _, bannedGeo := range cfg.BannedGeoLocations {
            if record.Country.ISOCode == bannedGeo {
                score += cfg.SuspicionWeights["banned_geo"]
            }
        }
    }
    
    // Check JA3 TLS fingerprint
    ja3Fingerprint := getJA3Fingerprint(r)
    if !isKnownBrowserJA3(ja3Fingerprint) {
        score += cfg.SuspicionWeights["tls_mismatch"]
    }
    
    // Check User-Agent
    ua := strings.ToLower(r.Header.Get("User-Agent"))
    if ua == "" || strings.Contains(ua, "curl") || strings.Contains(ua, "python") {
        score += cfg.SuspicionWeights["no_user_agent"]
    }
    if strings.Contains(ua, "headless") {
        score += cfg.SuspicionWeights["headless_browser"]
    }
    
    // Check fingerprint data
    if fp, exists := fingerprintStore.Data[clientIP]; exists {
        if fp.Webdriver {
            score += cfg.SuspicionWeights["headless_browser"]
        }
        if !fp.ChromeExists && strings.Contains(ua, "chrome") {
            score += cfg.SuspicionWeights["headless_browser"]
        }
    }
    
    return score >= cfg.SuspicionThreshold, score
}
```

#### JWT Token Verification

```go
func isVerified(r *http.Request) bool {
    cookie, err := r.Cookie("janus_token")
    if err != nil {
        return false
    }
    
    token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
        return jwtSecret, nil
    })
    
    if !token.Valid {
        return false
    }
    
    claims := token.Claims.(jwt.MapClaims)
    
    // Verify IP binding
    if claims["ip"].(string) != getClientIP(r) {
        return false
    }
    
    return true
}
```

### 6.3 Challenge System (`internal/challenge/challenge.go`)

#### Challenge Generation

Challenges are dynamically generated based on device type and risk score:

```go
func GenerateChallenge(cfg *config.JanusConfig, isMobile bool, riskScore int, history int) (*types.Challenge, int) {
    nonce := generateNonce()
    seed := generateSeed()
    
    // Set difficulty based on device
    baseDifficulty := cfg.DesktopDifficulty
    baseIterations := cfg.DesktopIterations
    challengeType := "pow"
    
    if isMobile {
        baseDifficulty = cfg.MobileDifficulty
        baseIterations = cfg.MobileIterations
    }
    
    // Adjust difficulty based on risk
    difficulty := baseDifficulty
    if riskScore < 20 && history > 2 {
        difficulty = 0  // Trusted user - minimal challenge
    } else if riskScore > 80 {
        difficulty = baseDifficulty + 2  // High risk - harder challenge
    }
    
    // Select challenge type based on risk
    if riskScore > 60 {
        if riskScore % 2 == 0 {
            challengeType = "image"  // Image puzzle
        } else {
            challengeType = "logic"  // Logic question
        }
    }
    
    return &types.Challenge{
        Nonce:      nonce,
        Iterations: baseIterations,
        Seed:       seed,
        Type:       challengeType,
        Difficulty: difficulty,
    }
}
```

#### Challenge Types

| Type | Description | When Used |
|------|-------------|-----------|
| `pow` | Proof-of-Work (SHA-256 with leading zeros) | Default, low-risk users |
| `image` | Image puzzle (click the cat) | Medium-high risk |
| `logic` | Logic question (what is 2+2?) | Medium-high risk |

#### Proof Verification

The proof format is: `nonce|iteration|timestamp|clientIP|seed[|canvasHash]`

```go
func VerifyChallenge(proof, expectedNonce, expectedClientIP, expectedSeed string, 
                     isMobile bool, canvasHash string, cfg *config.JanusConfig) bool {
    parts := strings.Split(proof, "|")
    
    // Validate part count
    if isMobile && len(parts) != 5 {
        return false
    }
    if !isMobile && len(parts) != 6 {
        return false
    }
    
    // Extract components
    nonce, iteration, timestamp, clientIP, seed := parts[0], parts[1], parts[2], parts[3], parts[4]
    
    // Verify nonce, IP, seed match
    if nonce != expectedNonce || clientIP != expectedClientIP || seed != expectedSeed {
        return false
    }
    
    // Verify canvas hash (desktop only)
    if !isMobile && parts[5] != canvasHash {
        return false
    }
    
    // Verify timestamp freshness (within 5 minutes)
    ts, _ := time.Parse(time.RFC3339, timestamp)
    if time.Since(ts) > 5*time.Minute {
        return false
    }
    
    // Verify Proof-of-Work (leading zero bits)
    hash := sha256.Sum256([]byte(proof))
    return hasLeadingZeroBits(hash[:], cfg.DesktopDifficulty)
}

func hasLeadingZeroBits(hash []byte, zeroBits int) bool {
    fullBytes := zeroBits / 8
    extraBits := zeroBits % 8
    
    for i := 0; i < fullBytes; i++ {
        if hash[i] != 0 {
            return false
        }
    }
    
    if extraBits > 0 {
        mask := byte(0xFF << (8 - extraBits))
        return (hash[fullBytes] & mask) == 0
    }
    return true
}
```

### 6.4 Data Types (`internal/types/types.go`)

```go
// Fingerprint represents collected browser fingerprint data
type Fingerprint struct {
    ClientIP      string `json:"client_ip"`
    Plugins       string `json:"plugins"`
    HardwareCon   int    `json:"hardware_concurrency"`
    Webdriver     bool   `json:"webdriver"`           // navigator.webdriver detection
    ChromeExists  bool   `json:"chrome_exists"`       // window.chrome existence
    CanvasHash    string `json:"canvas_hash"`         // Canvas fingerprint
    ScreenRes     string `json:"screen_resolution"`
    ColorDepth    int    `json:"color_depth"`
    Fonts         string `json:"fonts"`
    WebGLRenderer string `json:"webgl_renderer"`      // GPU info
    JA3           string `json:"ja3"`
    Screen        struct {
        Width  int `json:"width"`
        Height int `json:"height"`
    } `json:"screen"`
    Timezone  string `json:"timezone"`
    JSEnabled bool   `json:"jsEnabled"`
    IsMobile  bool   `json:"isMobile"`
}

// Challenge represents a verification challenge
type Challenge struct {
    Nonce      string
    Iterations int
    Seed       string
    Type       string  // "pow", "image", "logic"
    Difficulty int     // Number of leading zero bits required
}
```

### 6.5 Redis Store (`internal/store/store.go`)

```go
type Session struct {
    VerifiedAt              time.Time `json:"verifiedAt"`
    LastSeen                time.Time `json:"lastSeen"`
    HasScrolled             bool      `json:"hasScrolled"`
    HasNaturalMouseMovement bool      `json:"hasNaturalMouseMovement"`
    PagesViewed             int       `json:"pagesViewed"`
    NavigationPath          []string  `json:"navigationPath"`
}

type Store struct {
    rdb *redis.Client
}

// Rate limiting using Redis INCR with expiry
func (st *Store) IsRateLimited(identifier string, limit int) (bool, error) {
    key := "ratelimit:" + identifier
    
    pipe := st.rdb.Pipeline()
    count := pipe.Incr(ctx, key)
    pipe.ExpireNX(ctx, key, 1*time.Minute)  // 1-minute window
    
    _, err := pipe.Exec(ctx)
    return count.Val() > int64(limit), err
}

// Nonce management for replay protection
func (st *Store) CreateNonce(ttl time.Duration) (string, error) {
    nonce := uuid.New().String()
    key := "nonce:" + nonce
    err := st.rdb.Set(ctx, key, "valid", ttl).Err()
    return nonce, err
}

func (st *Store) ValidateNonce(nonce string) bool {
    key := "nonce:" + nonce
    err := st.rdb.GetDel(ctx, key).Err()  // Get and delete (one-time use)
    return err == nil
}
```

### 6.6 Configuration (`internal/config/config.go`)

```go
type JanusConfig struct {
    DesktopIterations  int            `yaml:"desktop_iterations"`   // Max PoW iterations
    MobileIterations   int            `yaml:"mobile_iterations"`
    DesktopDifficulty  int            `yaml:"desktop_difficulty"`   // Leading zero bits
    MobileDifficulty   int            `yaml:"mobile_difficulty"`
    WhitelistUA        []string       `yaml:"whitelist_ua"`         // Allowed User-Agents
    WhitelistIPs       []string       `yaml:"whitelist_ips"`        // Allowed IPs
    BlacklistedIPs     []string       `yaml:"blacklisted_ips"`      // Blocked IPs
    BannedGeoLocations []string       `yaml:"banned_geo_locations"` // Country codes
    SuspicionThreshold int            `yaml:"suspicion_threshold"`  // Score to flag
    SuspicionWeights   map[string]int `yaml:"suspicion_weights"`    // Signal weights
    RedisAddr          string         `yaml:"redis_addr"`
    RateLimit          struct {
        RequestsPerMinute int `yaml:"requests_per_minute"`
        Burst             int `yaml:"burst"`
    } `yaml:"rate_limit"`
}

func DefaultConfig() *JanusConfig {
    return &JanusConfig{
        DesktopIterations:  5000,
        MobileIterations:   5000,
        DesktopDifficulty:  8,    // 8 leading zero bits
        MobileDifficulty:   6,    // Easier for mobile
        WhitelistUA:        []string{"chrome", "firefox", "safari", "edge"},
        WhitelistIPs:       []string{"127.0.0.1", "::1"},
        SuspicionThreshold: 50,
        SuspicionWeights: map[string]int{
            "blacklisted_ip":        100,
            "banned_geo":            100,
            "tls_mismatch":          30,
            "no_user_agent":         40,
            "headless_browser":      50,
            "missing_headers":       20,
            "header_order_mismatch": 20,
            "no_fingerprint":        30,
        },
    }
}
```

---

## 7. Workflow & Data Flow

### Complete Request Lifecycle

```
┌────────────────────────────────────────────────────────────────────────────────┐
│                              JANUS REQUEST FLOW                                │
└────────────────────────────────────────────────────────────────────────────────┘

User Browser                    Janus Server                    Origin Server
     │                               │                               │
     │  1. GET /protected-page       │                               │
     │──────────────────────────────>│                               │
     │                               │                               │
     │                         ┌─────┴─────┐                         │
     │                         │ Check for │                         │
     │                         │janus_token│                         │
     │                         │ cookie    │                         │
     │                         └─────┬─────┘                         │
     │                               │                               │
     │                        [No Valid Token]                       │
     │                               │                               │
     │  2. Return challenge.html     │                               │
     │<──────────────────────────────│                               │
     │                               │                               │
     │  3. Load sensor.js            │                               │
     │──────────────────────────────>│                               │
     │<──────────────────────────────│                               │
     │                               │                               │
     │ ┌───────────────────────┐     │                               │
     │ │ Collect Fingerprint:  │     │                               │
     │ │ - Canvas hash         │     │                               │
     │ │ - WebGL renderer      │     │                               │
     │ │ - Screen resolution   │     │                               │
     │ │ - Plugins             │     │                               │
     │ │ - Timezone            │     │                               │
     │ │ - Hardware concurrency│     │                               │
     │ │ - navigator.webdriver │     │                               │
     │ │ - window.chrome       │     │                               │
     │ └───────────────────────┘     │                               │
     │                               │                               │
     │  4. POST /janus/fingerprint   │                               │
     │──────────────────────────────>│                               │
     │                               │  Store fingerprint            │
     │                               │  by IP address                │
     │  200 OK                       │                               │
     │<──────────────────────────────│                               │
     │                               │                               │
     │  5. GET /janus/challenge      │                               │
     │──────────────────────────────>│                               │
     │                               │ ┌────────────────────┐        │
     │                               │ │ Check fingerprint  │        │
     │                               │ │ Calculate risk     │        │
     │                               │ │ Generate challenge │        │
     │                               │ │ Store in cache     │        │
     │                               │ └────────────────────┘        │
     │  {nonce, seed, difficulty}    │                               │
     │<──────────────────────────────│                               │
     │                               │                               │
     │ ┌───────────────────────┐     │                               │
     │ │ Solve Challenge:      │     │                               │
     │ │ For i = 0 to max:     │     │                               │
     │ │   proof = compose()   │     │                               │
     │ │   hash = SHA256(proof)│     │                               │
     │ │   if hasZeroBits():   │     │                               │
     │ │     break             │     │                               │
     │ └───────────────────────┘     │                               │
     │                               │                               │
     │  6. POST /janus/verify        │                               │
     │  {nonce, proof}               │                               │
     │──────────────────────────────>│                               │
     │                               │ ┌────────────────────┐        │
     │                               │ │ Verify:            │        │
     │                               │ │ - Nonce matches    │        │
     │                               │ │ - IP matches       │        │
     │                               │ │ - Timestamp valid  │        │
     │                               │ │ - Hash has zeros   │        │
     │                               │ │ - Canvas matches   │        │
     │                               │ └────────────────────┘        │
     │                               │                               │
     │  Set-Cookie: janus_token=JWT  │                               │
     │  {status: "success"}          │                               │
     │<──────────────────────────────│                               │
     │                               │                               │
     │  7. Redirect to /             │                               │
     │  (with janus_token cookie)    │                               │
     │──────────────────────────────>│                               │
     │                               │ ┌────────────────────┐        │
     │                               │ │ Validate JWT:      │        │
     │                               │ │ - Signature OK     │        │
     │                               │ │ - Not expired      │        │
     │                               │ │ - IP matches       │        │
     │                               │ └────────────────────┘        │
     │                               │                               │
     │                               │  Forward to origin ──────────>│
     │                               │                               │
     │  8. Protected content         │<──────────────────────────────│
     │<──────────────────────────────│                               │
     │                               │                               │
```

### Challenge Types Flow

```
               ┌─────────────────────┐
               │ Risk Score Analysis │
               └──────────┬──────────┘
                          │
          ┌───────────────┼───────────────┐
          │               │               │
     Low Risk        Medium Risk      High Risk
    (score < 20)   (score 20-60)    (score > 60)
          │               │               │
     ┌────┴────┐     ┌────┴────┐     ┌────┴────┐
     │Invisible│     │  PoW    │     │ Interactive│
     │Challenge│     │Challenge│     │ Challenge  │
     │(diff=0) │     │(default)│     │(image/logic)│
     └─────────┘     └─────────┘     └────────────┘
```

---

## 8. Configuration

### config.yaml Structure

```yaml
# Challenge difficulty settings
desktop_iterations: 5000    # Maximum iterations for PoW
mobile_iterations: 5000     # Lower for mobile devices
desktop_difficulty: 8       # Leading zero bits (8 = ~256 average attempts)
mobile_difficulty: 6        # Easier for mobile (6 = ~64 average attempts)

# Whitelists
whitelist_ua:
  - chrome
  - firefox
  - safari
  - edge
whitelist_ips:
  - 127.0.0.1
  - "::1"

# Blacklists
blacklisted_ips: []
banned_geo_locations: []    # ISO country codes like "RU", "CN"

# Scoring thresholds
suspicion_threshold: 50     # Score at which to flag as suspicious

# Signal weights for scoring
suspicion_weights:
  blacklisted_ip: 100       # Immediate block
  banned_geo: 80            # High priority block
  tls_mismatch: 30          # JA3 fingerprint mismatch
  no_user_agent: 40         # Missing/suspicious UA
  headless_browser: 50      # Detected automation
  missing_headers: 20       # Missing browser headers
  header_order_mismatch: 20 # Unusual header ordering
  no_fingerprint: 30        # No JS fingerprint submitted

# Infrastructure
redis_addr: "redis:6379"

# Rate limiting
rate_limit:
  requests_per_minute: 60
  burst: 10
```

### Difficulty Explained

The `difficulty` parameter specifies how many leading zero bits the SHA-256 hash must have:

| Difficulty | Leading Zero Bits | Avg. Attempts | Time (est.) |
|------------|-------------------|---------------|-------------|
| 0 | 0 | 1 | instant |
| 4 | 4 | 16 | < 10ms |
| 8 | 8 | 256 | ~50ms |
| 12 | 12 | 4,096 | ~500ms |
| 16 | 16 | 65,536 | ~5s |

---

## 9. Frontend Components

### 9.1 Challenge Page (`assets/challenge.html`)

```html
<!DOCTYPE html>
<html>
<head>
    <title>JANUS Verification</title>
    <script src="/sensor.js"></script>
</head>
<body>
    <h1>Verifying your request...</h1>
    <p id="status">Collecting fingerprint and processing challenge...</p>
    <div id="challenge-ui" style="margin-top:2em;"></div>
    <script>
        // Override console.log to show status updates
        console.log = (function (origLog) {
            return function (message) {
                origLog.apply(console, arguments);
                const status = document.getElementById('status');
                if (message.includes('Fingerprint submitted')) {
                    status.textContent = 'Fingerprint submitted, fetching challenge...';
                } else if (message.includes('Received challenge')) {
                    status.textContent = 'Challenge received, verifying...';
                } else if (message.includes('Verification successful')) {
                    status.textContent = 'Verification successful, redirecting...';
                }
            };
        })(console.log);
    </script>
</body>
</html>
```

### 9.2 Sensor Script (`assets/sensor.js`)

The sensor script handles fingerprint collection and challenge solving:

#### Fingerprint Collection

```javascript
async function collectFingerprint() {
    // Canvas fingerprint
    const canvas = document.createElement('canvas');
    const ctx = canvas.getContext('2d');
    canvas.width = 200;
    canvas.height = 50;
    ctx.textBaseline = 'top';
    ctx.font = '14px Arial';
    ctx.fillText('fingerprint', 2, 2);
    const canvasHash = canvas.toDataURL();

    // Plugin detection
    const plugins = Array.from(navigator.plugins).map(p => p.name).join(',');
    
    // Screen info
    const screenRes = `${screen.width}x${screen.height}`;
    const colorDepth = screen.colorDepth;
    
    // Font detection
    const fonts = (function () {
        const testFonts = ['Arial', 'Times New Roman', 'Helvetica'];
        return testFonts.filter(font => 
            document.fonts.check(`12px "${font}"`)
        ).join(',');
    })();
    
    // WebGL renderer (GPU identification)
    const webgl = (function () {
        const gl = document.createElement('canvas').getContext('webgl');
        if (!gl) return 'no-webgl';
        return gl.getParameter(gl.RENDERER);
    })();
    
    // Mobile detection
    const isMobile = /Mobi|Android/i.test(navigator.userAgent);
    
    const fingerprint = {
        plugins: plugins,
        hardwareCon: navigator.hardwareConcurrency || 0,
        webdriver: !!navigator.webdriver,        // Bot detection!
        chromeExists: !!window.chrome,           // Headless Chrome detection
        canvas_Hash: canvasHash,
        screenRes: screenRes,
        colorDepth: colorDepth,
        fonts: fonts,
        webglRenderer: webgl,
        ja3: 'unknown-ja3',
        screen: { width: screen.width, height: screen.height },
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
        jsEnabled: true,
        isMobile: isMobile
    };
    
    // Submit fingerprint
    await fetch('/janus/fingerprint', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(fingerprint)
    });
}
```

#### Challenge Solving

```javascript
// Solve Proof-of-Work challenge
const { nonce, iterations, seed, clientIP, difficulty } = challenge;
const timestamp = new Date().toISOString();
let proof;
const maxIterations = Math.min(iterations, isMobile ? 1000 : 5000);

for (let i = 0; i < maxIterations; i++) {
    proof = `${nonce}|${i}|${timestamp}|${clientIP}|${seed}`;
    if (!isMobile) {
        proof += `|${canvasHash}`;
    }
    
    const hash = await crypto.subtle.digest('SHA-256', 
        new TextEncoder().encode(proof));
    const hashArray = new Uint8Array(hash);
    
    if (hasLeadingZeroBits(hashArray, difficulty)) {
        break;  // Found valid proof!
    }
}

// Leading zero bits checker
function hasLeadingZeroBits(hash, zeroBits) {
    const fullBytes = Math.floor(zeroBits / 8);
    const extraBits = zeroBits % 8;
    
    for (let i = 0; i < fullBytes; i++) {
        if (hash[i] !== 0) return false;
    }
    
    if (extraBits > 0) {
        const mask = 0xFF << (8 - extraBits);
        return (hash[fullBytes] & mask) === 0;
    }
    return true;
}
```

#### Interactive Challenges

```javascript
// Image challenge
if (challenge.type === 'image') {
    challengeUI.innerHTML = `
        <b>Image Puzzle:</b> Click the cat image to continue.
        <img id="cat-img" src="https://cataas.com/cat?width=120" 
             style="cursor:pointer; max-width:120px;">
    `;
    document.getElementById('cat-img').onclick = async function() {
        await verifyProof('image-solved');
    };
}

// Logic challenge
if (challenge.type === 'logic') {
    challengeUI.innerHTML = `
        <b>Logic Question:</b> What is 2 + 2? 
        <input id="logic-answer" type="text" size="4"> 
        <button id="logic-btn">Submit</button>
    `;
    document.getElementById('logic-btn').onclick = async function() {
        const answer = document.getElementById('logic-answer').value;
        if (answer.trim() === '4') {
            await verifyProof('logic-4');
        } else {
            alert('Try again!');
        }
    };
}
```

---

## 10. Backend Components

### 10.1 HTTP Handlers

#### Fingerprint Handler

```go
func HandleFingerprint(store *types.FingerprintStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var fp types.Fingerprint
        json.NewDecoder(r.Body).Decode(&fp)
        
        fp.ClientIP = getClientIP(r)
        
        store.Lock()
        store.Data[fp.ClientIP] = fp
        store.Unlock()
        
        w.WriteHeader(http.StatusOK)
    }
}
```

#### Challenge Handler

```go
func handleChallenge(w http.ResponseWriter, r *http.Request) {
    clientIP := getClientIP(r)
    
    // Require fingerprint first
    fingerprintStore.RLock()
    fp, hasFingerprint := fingerprintStore.Data[clientIP]
    fingerprintStore.RUnlock()
    
    if !hasFingerprint {
        http.Error(w, "No fingerprint", http.StatusBadRequest)
        return
    }
    
    // Calculate risk score
    _, riskScore := isSuspicious(r, loadedConfig)
    
    // Generate challenge based on risk
    chal, _ := challenge.GenerateChallenge(loadedConfig, fp.IsMobile, riskScore, 0)
    
    // Store challenge with 5-minute expiry
    challengeStore.Lock()
    challengeStore.data[clientIP+chal.Nonce] = struct {
        Challenge *types.Challenge
        Expires   time.Time
    }{Challenge: chal, Expires: time.Now().Add(5 * time.Minute)}
    challengeStore.Unlock()
    
    // Return challenge to client
    json.NewEncoder(w).Encode(map[string]interface{}{
        "nonce":      chal.Nonce,
        "iterations": chal.Iterations,
        "seed":       chal.Seed,
        "clientIP":   clientIP,
        "type":       chal.Type,
        "difficulty": chal.Difficulty,
    })
}
```

#### Verify Handler

```go
func handleVerify(w http.ResponseWriter, r *http.Request) {
    clientIP := getClientIP(r)
    
    var req struct {
        Nonce string `json:"nonce"`
        Proof string `json:"proof"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    // Get stored fingerprint
    fingerprintStore.RLock()
    fp, hasFingerprint := fingerprintStore.Data[clientIP]
    fingerprintStore.RUnlock()
    
    if !hasFingerprint {
        http.Error(w, "No fingerprint", http.StatusBadRequest)
        return
    }
    
    // Get stored challenge
    challengeStore.RLock()
    stored, exists := challengeStore.data[clientIP+req.Nonce]
    challengeStore.RUnlock()
    
    if !exists || time.Now().After(stored.Expires) {
        http.Error(w, "No valid challenge", http.StatusBadRequest)
        return
    }
    
    // Verify the proof
    if !challenge.VerifyChallenge(req.Proof, req.Nonce, clientIP, 
                                   stored.Challenge.Seed, fp.IsMobile, 
                                   fp.CanvasHash, loadedConfig) {
        http.Error(w, "Verification failed", http.StatusUnauthorized)
        return
    }
    
    // Clean up used challenge
    challengeStore.Lock()
    delete(challengeStore.data, clientIP+req.Nonce)
    challengeStore.Unlock()
    
    // Issue JWT token
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "ip":  clientIP,
        "exp": time.Now().Add(24 * time.Hour).Unix(),
    })
    tokenString, _ := token.SignedString(jwtSecret)
    
    // Set secure cookie
    http.SetCookie(w, &http.Cookie{
        Name:     "janus_token",
        Value:    tokenString,
        Path:     "/",
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteStrictMode,
        MaxAge:   24 * 60 * 60,  // 24 hours
    })
    
    json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
```

### 10.2 JA3 TLS Fingerprinting

JA3 is a method for creating SSL/TLS client fingerprints based on TLS handshake parameters:

```go
func getJA3Fingerprint(r *http.Request) string {
    if r.TLS == nil {
        return "no-tls"
    }
    
    var parts []string
    
    // TLS version (e.g., 771 for TLS 1.2)
    parts = append(parts, strconv.Itoa(int(r.TLS.Version)))
    
    // Cipher suite
    var ciphers []string
    ciphers = append(ciphers, strconv.Itoa(int(r.TLS.CipherSuite)))
    parts = append(parts, strings.Join(ciphers, "-"))
    
    // Extensions, elliptic curves, and elliptic curve point formats
    // (Simplified - real JA3 includes more data from ClientHello)
    parts = append(parts, "0-23-65281-10-11-35-16-5-13-18-51-45-43-27")
    parts = append(parts, "29-23-24")
    parts = append(parts, "0")
    
    ja3String := strings.Join(parts, ",")
    ja3Hash := fmt.Sprintf("%x", md5.Sum([]byte(ja3String)))
    
    return ja3Hash
}

// Known browser JA3 fingerprints
func isKnownBrowserJA3(ja3 string) bool {
    known := []string{
        "b2fa5d224d65e7c692fd46a0f52fce6b",  // Chrome
        // ... other known browser fingerprints
    }
    for _, k := range known {
        if ja3 == k {
            return true
        }
    }
    return false
}
```

---

## 11. Security Mechanisms

### 11.1 Multi-Layer Defense

```
Layer 1: Rate Limiting
├── IP-based request limiting (default: 60/min)
├── Redis-backed for distributed deployments
└── Burst handling for legitimate traffic spikes

Layer 2: IP/Geo Reputation
├── IP blacklist checking
├── GeoIP-based country blocking
└── CIDR range blocking support

Layer 3: TLS Fingerprinting (JA3)
├── Extract TLS handshake parameters
├── Compare against known browser fingerprints
└── Flag inconsistencies between UA and JA3

Layer 4: Browser Fingerprinting
├── Canvas fingerprint (GPU/driver-specific)
├── WebGL renderer identification
├── Plugin enumeration
├── Screen resolution & color depth
├── Hardware concurrency
├── Font availability
├── Timezone detection

Layer 5: Bot Detection
├── navigator.webdriver check
├── window.chrome existence check
├── Headless browser UA detection
├── Missing/suspicious header detection
├── Header order anomalies

Layer 6: Challenge-Response
├── Proof-of-Work (SHA-256 hash with leading zeros)
├── Proof-of-Render (canvas hash verification)
├── Interactive challenges (image/logic puzzles)
├── Device-appropriate difficulty

Layer 7: Token Security
├── JWT-based session tokens
├── IP binding in token claims
├── 24-hour expiration
├── HttpOnly, Secure, SameSite=Strict cookies
├── Challenge nonce replay protection
```

### 11.2 Bot Detection Signals

| Signal | Detection Method | Bot Indicator |
|--------|------------------|---------------|
| `navigator.webdriver` | JS check | `true` in automation frameworks |
| `window.chrome` | JS check | Missing in headless Chrome |
| User-Agent | HTTP header | "headless", "curl", "python" |
| Accept header | HTTP header | Missing or minimal |
| Header order | HTTP analysis | Non-browser patterns |
| Canvas hash | Rendered canvas | Empty/error indicates missing GPU |
| WebGL renderer | WebGL query | "no-webgl" or SwiftShader |
| JA3 hash | TLS handshake | Non-browser TLS clients |

### 11.3 Replay Protection

```
Challenge Lifecycle:
1. Generate unique nonce + seed for each challenge
2. Store with 5-minute expiry
3. On verification: delete immediately (one-time use)
4. Bind to IP address
5. Include timestamp in proof (checked within 5 min window)
```

### 11.4 JWT Token Security

```go
// Token structure
{
    "ip": "192.168.1.1",      // IP binding
    "exp": 1740355200,        // 24-hour expiration
    "iat": 1740268800         // Issued at
}

// Cookie settings
http.SetCookie(w, &http.Cookie{
    Name:     "janus_token",
    Value:    tokenString,
    Path:     "/",
    HttpOnly: true,           // No JS access
    Secure:   true,           // HTTPS only
    SameSite: http.SameSiteStrictMode,  // CSRF protection
    MaxAge:   24 * 60 * 60,
})
```

---

## 12. API Reference

### Endpoints

#### POST /janus/fingerprint

Submit browser fingerprint data.

**Request:**
```json
{
    "plugins": "Chrome PDF Plugin,Chrome PDF Viewer",
    "hardware_concurrency": 8,
    "webdriver": false,
    "chrome_exists": true,
    "canvas_hash": "data:image/png;base64,iVBORw0...",
    "screen_resolution": "1920x1080",
    "color_depth": 24,
    "fonts": "Arial,Helvetica,Times New Roman",
    "webgl_renderer": "ANGLE (NVIDIA GeForce GTX 1080...)",
    "timezone": "America/New_York",
    "isMobile": false
}
```

**Response:** `200 OK` (empty body)

---

#### GET /janus/challenge

Request a verification challenge.

**Prerequisites:** Fingerprint must be submitted first.

**Response:**
```json
{
    "nonce": "dGVzdC1ub25jZS1iYXNlNjQ=",
    "iterations": 5000,
    "seed": "c2VlZC1iYXNlNjQ=",
    "clientIP": "192.168.1.1",
    "type": "pow",
    "difficulty": 8
}
```

**Challenge Types:**
- `pow` - Proof-of-Work (find hash with leading zeros)
- `image` - Click image puzzle
- `logic` - Answer simple math question

---

#### POST /janus/verify

Submit challenge solution.

**Request:**
```json
{
    "nonce": "dGVzdC1ub25jZS1iYXNlNjQ=",
    "proof": "dGVzdC1ub25jZS1iYXNlNjQ=|42|2026-02-23T12:00:00Z|192.168.1.1|c2VlZC1iYXNlNjQ=|data:image/png..."
}
```

**Response (Success):**
```json
{
    "status": "success"
}
```
+ `Set-Cookie: janus_token=<JWT>`

**Response (Failure):**
```
HTTP 401 Unauthorized
Verification failed
```

---

## 13. Docker & Deployment

### Dockerfile

```dockerfile
# Multi-stage build
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/janus ./cmd/janus

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y openssl ca-certificates
WORKDIR /app
COPY --from=build /app/janus /app/janus
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh
EXPOSE 8080 8081
ENTRYPOINT ["/app/entrypoint.sh"]
```

### docker-compose.yml

```yaml
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    container_name: janus-redis
    restart: unless-stopped
    ports:
      - "6379:6379"
      
  janus:
    build: .
    container_name: janus-server
    depends_on:
      - redis
    environment:
      - REDIS_ADDR=redis:6379
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./GeoLite2-City.mmdb:/app/GeoLite2-City.mmdb:ro
      - ./cert.pem:/app/cert.pem:ro
      - ./key.pem:/app/key.pem:ro
    command: ["/app/janus"]
```

### entrypoint.sh

```bash
#!/usr/bin/env bash
set -euo pipefail
cd /app

# Generate self-signed certs if not present
if [ ! -f cert.pem ] || [ ! -f key.pem ]; then
  echo "Generating self-signed certs..."
  openssl req -x509 -newkey rsa:2048 -keyout key.pem \
    -out cert.pem -days 365 -nodes -subj "/CN=localhost"
fi

echo "Starting janus server"
exec ./janus
```

### Deployment Options

#### Development (Local)
```powershell
# Generate certs
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes

# Run directly
go run ./cmd/janus
```

#### Docker Compose (Staging)
```bash
docker-compose up --build
```

#### Kubernetes (Production)
```yaml
# Recommended setup
- Horizontal Pod Autoscaler for Janus pods
- Redis cluster for session storage
- Ingress with cert-manager for TLS
- Prometheus ServiceMonitor for metrics
```

---

## 14. Testing

### Manual Testing Flow

1. **Start the server:**
```powershell
go run ./cmd/janus
```

2. **Visit in browser:**
```
https://localhost:8080/
```

3. **Observe the flow:**
   - Challenge page loads
   - Fingerprint collected
   - Challenge issued and solved
   - Redirect to protected content

### Automated Test Script (Bash)

```bash
#!/usr/bin/env bash
BASE_URL="https://localhost:8080"

# 1. Submit fingerprint
curl -ks -X POST "$BASE_URL/janus/fingerprint" \
  -H "Content-Type: application/json" \
  -d '{"client_ip":"127.0.0.1","canvas_hash":"test-canvas","isMobile":false}'

# 2. Get challenge
CHAL=$(curl -ks "$BASE_URL/janus/challenge")
NONCE=$(echo $CHAL | jq -r '.nonce')
SEED=$(echo $CHAL | jq -r '.seed')
CLIENTIP=$(echo $CHAL | jq -r '.clientIP')

# 3. Craft proof (with difficulty=0 for testing)
TS=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
PROOF="$NONCE|1|$TS|$CLIENTIP|$SEED|test-canvas"

# 4. Submit verification
curl -ks -X POST "$BASE_URL/janus/verify" \
  -H "Content-Type: application/json" \
  -d "{\"nonce\":\"$NONCE\",\"proof\":\"$PROOF\"}"
```

### Testing with Zero Difficulty

For testing, set difficulty to 0 in `config.yaml`:
```yaml
desktop_difficulty: 0
mobile_difficulty: 0
```

This makes the PoW trivial, allowing quick verification testing.

---

## 15. Roadmap & Future Enhancements

### Current Status (MVP 0.1)

- [x] Basic middleware structure
- [x] Fingerprint collection (canvas, WebGL, plugins)
- [x] Proof-of-Work challenge system
- [x] JWT token issuance
- [x] Redis session store
- [x] Rate limiting
- [x] GeoIP blocking
- [x] JA3 fingerprinting (basic)
- [x] Docker support

### Planned Enhancements

#### Short-term (v0.2)

- [ ] **Behavioral Analytics**
  - Mouse movement entropy analysis
  - Scroll pattern detection
  - Click timing analysis
  
- [ ] **Mobile Sensor Support**
  - Accelerometer data collection
  - Gyroscope pattern analysis
  - Touch pressure curves

- [ ] **Zombie Detection**
  - Track pages visited vs. scroll events
  - Flag users who navigate but never interact

#### Medium-term (v0.3)

- [ ] **ML Scoring Integration**
  - Train anomaly detection model
  - Real-time inference for risk scoring
  - A/B testing framework

- [ ] **Admin Dashboard**
  - Real-time metrics visualization
  - Session browser
  - Manual whitelist/unban interface

- [ ] **Observability**
  - Prometheus metrics export
  - Grafana dashboard templates
  - Structured JSON logging

#### Long-term (v1.0)

- [ ] **WebSocket Challenge Channel**
  - Real-time challenge status updates
  - Progressive difficulty adjustment

- [ ] **Accessibility Improvements**
  - Audio challenge fallback
  - Email OTP verification option
  - Screen reader compatibility

- [ ] **Privacy Enhancements**
  - Consent flow for fingerprinting
  - Data retention policies
  - GDPR compliance tools

- [ ] **Enterprise Features**
  - Multi-tenant support
  - Custom challenge branding
  - SLA monitoring

---

## 16. Glossary

| Term | Definition |
|------|------------|
| **Canvas Fingerprint** | A unique identifier generated by rendering text/graphics on an HTML canvas element. Different GPUs/drivers produce subtly different outputs. |
| **JA3** | A method for creating SSL/TLS client fingerprints based on specific fields in the TLS ClientHello message. |
| **Proof-of-Work (PoW)** | A computational challenge requiring the client to find an input that produces a hash with specific properties (e.g., leading zeros). |
| **Proof-of-Render (PoR)** | A challenge requiring the client to perform GPU-based rendering and submit a hash of the result, proving browser capability. |
| **WebGL Renderer** | Information about the GPU and driver used for 3D rendering, obtainable via WebGL API. Useful for fingerprinting. |
| **Headless Browser** | A web browser without a graphical user interface, often used for automation and scraping. |
| **navigator.webdriver** | A JavaScript property that is `true` when the browser is controlled by automation tools like Selenium or Puppeteer. |
| **Nonce** | A random number used once. In Janus, it prevents replay attacks by binding challenges to specific requests. |
| **JWT (JSON Web Token)** | A compact, URL-safe token format for securely transmitting information between parties. |
| **Zombie Detector** | A system that identifies users who pass initial verification but exhibit bot-like behavior (no scrolling, rapid navigation). |
| **Suspicion Score** | A weighted numeric score calculated from multiple signals. Higher scores indicate higher likelihood of automated traffic. |
| **GeoIP** | The process of determining geographic location based on IP address, using databases like MaxMind's GeoLite2. |
| **Rate Limiting** | Restricting the number of requests a client can make within a time window to prevent abuse. |
| **TLS** | Transport Layer Security - the cryptographic protocol used for HTTPS connections. |
| **HMAC** | Hash-based Message Authentication Code - used to verify data integrity and authenticity. |

---

## Quick Reference Card

### Running Janus

```powershell
# Development
go run ./cmd/janus

# Production (Docker)
docker-compose up -d
```

### Key Files

| File | Purpose |
|------|---------|
| `cmd/janus/main.go` | Entry point |
| `internal/middleware/janus.go` | Core logic |
| `internal/challenge/challenge.go` | Challenge system |
| `assets/sensor.js` | Client fingerprinting |
| `config.yaml` | Configuration |

### API Quick Reference

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/janus/fingerprint` | POST | Submit fingerprint |
| `/janus/challenge` | GET | Get challenge |
| `/janus/verify` | POST | Submit proof |

### Configuration Essentials

```yaml
desktop_difficulty: 8    # 1-16, higher = harder
suspicion_threshold: 50  # 0-100
rate_limit:
  requests_per_minute: 60
```

---

*This documentation was generated based on a comprehensive analysis of the Janus codebase as of February 2026.*
