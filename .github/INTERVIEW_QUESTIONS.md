# Janus Project - Interview Questions

## Go Backend & REST API Technical Interview Guide

This document contains interview questions based on the Janus bot-detection middleware project. Questions are organized by topic and difficulty level.

---

## Table of Contents

1. [Go Language Fundamentals](#1-go-language-fundamentals)
2. [HTTP & REST API Design](#2-http--rest-api-design)
3. [Middleware Architecture](#3-middleware-architecture)
4. [Concurrency & Thread Safety](#4-concurrency--thread-safety)
5. [Security Implementation](#5-security-implementation)
6. [System Design](#6-system-design)
7. [Code Review Questions](#7-code-review-questions)
8. [Scenario-Based Questions](#8-scenario-based-questions)

---

## 1. Go Language Fundamentals

### Easy

**Q1.1:** In the Janus middleware, we use `sync.RWMutex` for the fingerprint store:
```go
type FingerprintStore struct {
    sync.RWMutex
    Data map[string]Fingerprint
}
```
Why do we embed `sync.RWMutex` directly instead of having it as a named field? What's the difference?

<details>
<summary>Answer</summary>

Embedding `sync.RWMutex` directly promotes its methods (`Lock()`, `Unlock()`, `RLock()`, `RUnlock()`) to the struct level, allowing us to call `store.Lock()` instead of `store.mutex.Lock()`. This is a common Go idiom for cleaner code. The functionality is identical - it's purely syntactic convenience.
</details>

---

**Q1.2:** Explain the difference between these two map access patterns used in the codebase:
```go
// Pattern 1
fp, hasFingerprint := fingerprintStore.Data[clientIP]

// Pattern 2  
fp := fingerprintStore.Data[clientIP]
```

<details>
<summary>Answer</summary>

Pattern 1 uses the "comma ok" idiom which returns both the value and a boolean indicating if the key exists. This distinguishes between "key doesn't exist" and "key exists with zero value". Pattern 2 only returns the value (zero value if key doesn't exist). In Janus, we use Pattern 1 because we need to know if a fingerprint was actually submitted vs. just being empty.
</details>

---

**Q1.3:** Why does the `config.LoadConfig()` function return `(*JanusConfig, error)` instead of `(JanusConfig, error)`?

```go
func LoadConfig(path string) (*JanusConfig, error) {
    cfg := DefaultConfig()
    // ...
    return cfg, nil
}
```

<details>
<summary>Answer</summary>

Returning a pointer avoids copying the entire struct on return. `JanusConfig` contains slices and maps which would need to be copied element-by-element if returned by value. Returning a pointer is more efficient and allows the caller to modify the config if needed. It also allows returning `nil` to indicate "no config" vs. an empty config.
</details>

---

### Medium

**Q1.4:** Look at this initialization pattern in `janus.go`:
```go
var loadedConfig *config.JanusConfig
var configOnce sync.Once

func JanusMiddleware(next http.Handler) http.Handler {
    configOnce.Do(func() {
        var err error
        loadedConfig, err = config.LoadConfig("config.yaml")
        // ...
    })
    // ...
}
```
Why use `sync.Once` here? What problem does it solve?

<details>
<summary>Answer</summary>

`sync.Once` ensures the configuration is loaded exactly once, even if `JanusMiddleware` is called concurrently from multiple goroutines. Without it, there's a race condition where multiple goroutines could try to load and assign the config simultaneously. `sync.Once` is idiomatic Go for lazy initialization of shared resources in concurrent code.
</details>

---

**Q1.5:** In the challenge verification, we see:
```go
ts, err := time.Parse(time.RFC3339, timestamp)
if err != nil || time.Since(ts) > 5*time.Minute || ts.After(time.Now().Add(1*time.Minute)) {
    return false
}
```
Why do we check both `time.Since(ts) > 5*time.Minute` AND `ts.After(time.Now().Add(1*time.Minute))`?

<details>
<summary>Answer</summary>

This is a security measure:
1. `time.Since(ts) > 5*time.Minute` - Prevents replay of old challenges (proof must be recent)
2. `ts.After(time.Now().Add(1*time.Minute))` - Prevents proofs with future timestamps (attackers might try to submit proofs with timestamps in the future to extend validity)

The 1-minute buffer for future timestamps allows for clock skew between client and server.
</details>

---

**Q1.6:** Explain what this goroutine does and why it's started in `init()`:
```go
func init() {
    go func() {
        for {
            time.Sleep(1 * time.Minute)
            challengeStore.Lock()
            for key, stored := range challengeStore.data {
                if time.Now().After(stored.Expires) {
                    delete(challengeStore.data, key)
                }
            }
            challengeStore.Unlock()
        }
    }()
}
```

<details>
<summary>Answer</summary>

This is a background garbage collector for expired challenges. It runs every minute and removes challenges that have exceeded their 5-minute TTL. This prevents memory leaks from accumulating unused challenges. It's started in `init()` to ensure it runs for the lifetime of the application. The mutex prevents race conditions with handlers that read/write challenges.
</details>

---

### Hard

**Q1.7:** Consider this code from `hasLeadingZeroBits`:
```go
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
Explain the bit manipulation logic. If `zeroBits = 10`, what exactly does this check?

<details>
<summary>Answer</summary>

For `zeroBits = 10`:
- `fullBytes = 10 / 8 = 1` - First byte must be all zeros
- `extraBits = 10 % 8 = 2` - First 2 bits of second byte must be zeros

The mask calculation for extraBits=2:
- `0xFF << (8 - 2)` = `0xFF << 6` = `0b11000000`
- `hash[1] & 0b11000000` must equal 0

This mask checks only the leftmost 2 bits. If a 256-bit SHA-256 hash starts with `00000000 00xxxxxx...`, it passes.
</details>

---

**Q1.8:** The `getClientIP` function handles both IPv4 and IPv6:
```go
func getClientIP(r *http.Request) string {
    if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
        ip := strings.Split(forwarded, ",")[0]
        return strings.TrimSpace(ip)
    }
    addr := r.RemoteAddr
    if strings.HasPrefix(addr, "[") {
        end := strings.LastIndex(addr, "]")
        if end != -1 {
            return addr[1:end]
        }
    }
    // ... IPv4 handling
}
```
Why check for `[` prefix? What format is this handling?

<details>
<summary>Answer</summary>

IPv6 addresses in Go's `RemoteAddr` are formatted as `[::1]:8080` (brackets around the address, then colon and port). IPv4 is just `127.0.0.1:8080`. The brackets are needed for IPv6 because the address itself contains colons, so without brackets you couldn't distinguish address colons from the port separator.

The code extracts just the IP by finding the closing bracket and slicing `[1:end]` to get `::1` from `[::1]:8080`.
</details>

---

## 2. HTTP & REST API Design

### Easy

**Q2.1:** The Janus API has these three endpoints:
```
POST /janus/fingerprint
GET  /janus/challenge
POST /janus/verify
```
Why is `/janus/challenge` a GET request while the others are POST?

<details>
<summary>Answer</summary>

Following REST principles:
- `GET /challenge` - Retrieves a new challenge resource; idempotent (calling multiple times is safe), has no request body needed
- `POST /fingerprint` - Creates/submits new fingerprint data; has a request body
- `POST /verify` - Submits proof for verification; has a request body, causes state change

GET is appropriate for challenge because it's essentially "give me a challenge" without sending data, and the client might need to retry if something fails.
</details>

---

**Q2.2:** The handleChallenge returns JSON like this:
```go
response := map[string]interface{}{
    "nonce":      chal.Nonce,
    "iterations": chal.Iterations,
    "seed":       chal.Seed,
    "clientIP":   clientIP,
    "type":       chal.Type,
    "difficulty": chal.Difficulty,
}
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(response)
```
Why do we explicitly set `Content-Type` header? What happens if we don't?

<details>
<summary>Answer</summary>

Without setting `Content-Type`, Go's `http.ResponseWriter` may try to detect the content type via `http.DetectContentType()`, which examines the first 512 bytes. For JSON, it might incorrectly detect as `text/plain` since JSON is plain text. Explicitly setting `application/json`:
1. Ensures browsers/clients parse it as JSON
2. Enables proper CORS handling
3. Allows clients to use `response.json()` without errors
4. Is semantically correct per HTTP standards
</details>

---

### Medium

**Q2.3:** The verify endpoint sets these cookie attributes:
```go
http.SetCookie(w, &http.Cookie{
    Name:     "janus_token",
    Value:    tokenString,
    Path:     "/",
    HttpOnly: true,
    Secure:   true,
    SameSite: http.SameSiteStrictMode,
    MaxAge:   24 * 60 * 60,
})
```
Explain each security attribute and why it's important.

<details>
<summary>Answer</summary>

- **HttpOnly: true** - Cookie cannot be accessed via JavaScript (`document.cookie`), preventing XSS attacks from stealing the token
- **Secure: true** - Cookie only sent over HTTPS, preventing interception on unencrypted connections
- **SameSite: Strict** - Cookie not sent with cross-site requests, preventing CSRF attacks
- **Path: "/" ** - Cookie valid for entire site
- **MaxAge: 86400** - 24-hour expiry, limits window if token is compromised

Together these implement defense-in-depth for session security.
</details>

---

**Q2.4:** The `handleFingerprint` has no response body:
```go
func HandleFingerprint(store *types.FingerprintStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var fp types.Fingerprint
        if err := json.NewDecoder(r.Body).Decode(&fp); err != nil {
            return  // Silent return!
        }
        fp.ClientIP = getClientIP(r)
        store.Lock()
        store.Data[fp.ClientIP] = fp
        store.Unlock()
        w.WriteHeader(http.StatusOK)
    }
}
```
Is this good API design? What improvements would you suggest?

<details>
<summary>Answer</summary>

Issues with current design:
1. Silent failure on JSON decode error - returns 200 OK implicitly
2. No error response to client on malformed input
3. No response body confirming what was stored

Improvements:
```go
if err := json.NewDecoder(r.Body).Decode(&fp); err != nil {
    http.Error(w, "Invalid JSON", http.StatusBadRequest)
    return
}
// After storing:
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]string{
    "status": "stored",
    "ip": fp.ClientIP,
})
```
</details>

---

**Q2.5:** The main router uses chi:
```go
r := chi.NewRouter()
r.Use(middleware.JanusMiddleware)
r.Get("/", handler)
r.Get("/sensor.js", serveJS)
```
But internally, Janus also has its own router:
```go
janusRouter = chi.NewRouter()
janusRouter.Post("/janus/fingerprint", ...)
janusRouter.Get("/janus/challenge", ...)
```
Why have two routers? Why not register all routes on the main router?

<details>
<summary>Answer</summary>

Separation of concerns:
1. The main router is controlled by the application - it decides what to protect
2. The Janus router handles internal API endpoints that the middleware needs
3. Janus endpoints must NOT be protected by Janus middleware (chicken-egg problem)

In the middleware:
```go
if strings.HasPrefix(r.URL.Path, "/janus/") {
    janusRouter.ServeHTTP(w, r)
    return  // Skip protection
}
```

The inner router is essentially an "escape hatch" that bypasses the protection layer. This pattern allows Janus to be a drop-in middleware without requiring the application to manually configure bypass routes.
</details>

---

### Hard

**Q2.6:** Design critique: The current API requires three round-trips for verification:
```
1. POST /janus/fingerprint
2. GET /janus/challenge  
3. POST /janus/verify
```
Could this be reduced to fewer round-trips? What are the trade-offs?

<details>
<summary>Answer</summary>

**Option 1: Combine fingerprint + challenge (2 round-trips)**
```
POST /janus/init - Submit fingerprint, get challenge back
POST /janus/verify - Submit proof
```
Trade-offs:
- ✅ One less round-trip
- ❌ Larger response payload
- ❌ Can't retry challenge without re-submitting fingerprint

**Option 2: Single round-trip**
```
POST /janus/verify - Submit fingerprint + proof together
```
Trade-offs:
- ✅ Minimal latency
- ❌ Challenge must be embedded in page (less dynamic)
- ❌ Harder to adjust difficulty based on fingerprint

**Current design rationale:**
- Fingerprint storage is decoupled from challenge generation
- Server can analyze fingerprint before deciding challenge type/difficulty
- Each step can fail independently with clear error handling
- Allows challenge retry without re-fingerprinting

The 3-step design prioritizes **flexibility and security** over raw performance.
</details>

---

## 3. Middleware Architecture

### Easy

**Q3.1:** What is the function signature of Go HTTP middleware and why?
```go
func JanusMiddleware(next http.Handler) http.Handler
```

<details>
<summary>Answer</summary>

This signature follows the **decorator pattern**:
- Takes `http.Handler` (the next handler in chain)
- Returns `http.Handler` (wrapping the original)

This allows chaining: `Middleware1(Middleware2(Middleware3(finalHandler)))`

When a request comes in:
1. Middleware1 runs its "before" logic
2. Calls `next.ServeHTTP()` to pass to Middleware2
3. After next returns, runs "after" logic

The `http.Handler` interface only requires `ServeHTTP(w, r)`, keeping it simple and composable.
</details>

---

**Q3.2:** In the middleware, we see this pattern:
```go
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // middleware logic
})
```
Why do we return `http.HandlerFunc` instead of a custom struct implementing `http.Handler`?

<details>
<summary>Answer</summary>

`http.HandlerFunc` is an adapter type:
```go
type HandlerFunc func(ResponseWriter, *Request)
func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) { f(w, r) }
```

It converts any function with the right signature into an `http.Handler`. This is more concise than:
```go
type janusHandler struct { next http.Handler }
func (j *janusHandler) ServeHTTP(w, r) { ... }
return &janusHandler{next: next}
```

Using `HandlerFunc` is idiomatic Go - less boilerplate for simple middleware that doesn't need struct state.
</details>

---

### Medium

**Q3.3:** Analyze this middleware flow:
```go
func JanusMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. Handle /janus/* routes internally
        if strings.HasPrefix(r.URL.Path, "/janus/") {
            janusRouter.ServeHTTP(w, r)
            return
        }
        
        // 2. Check rate limit
        if limited { return }
        
        // 3. Check verification
        if isVerified(r) {
            next.ServeHTTP(w, r)
            return
        }
        
        // 4. Issue challenge
        issueChallenge(w, r)
    })
}
```
Why is the order of these checks important? What would happen if we checked verification before handling `/janus/*` routes?

<details>
<summary>Answer</summary>

If verification was checked first:
1. User visits site → Not verified → Gets challenge page
2. Challenge page loads `/janus/fingerprint` → Would try to verify → Fail → Another challenge
3. **Infinite loop!** User can never complete verification

The `/janus/*` bypass MUST come first because:
- The verification flow itself needs those endpoints
- Rate limiting on API endpoints is handled separately within those handlers
- It's the "escape hatch" that allows unverified users to become verified

Current order ensures:
1. API endpoints always accessible (for verification flow)
2. Rate limiting protects against abuse
3. Verified users skip challenges
4. Unverified users get challenged
</details>

---

**Q3.4:** The middleware stores fingerprints in a package-level variable:
```go
var fingerprintStore = &types.FingerprintStore{Data: make(map[string]Fingerprint)}
```
What are the implications of this design? How would you change it for a distributed system?

<details>
<summary>Answer</summary>

**Current design implications:**
- Single-instance only (no horizontal scaling)
- Process restart loses all fingerprints
- Fast (in-memory)
- Simple to implement

**For distributed systems:**

1. **Redis-backed store** (like the rate limiter already uses):
```go
type RedisFingerrintStore struct {
    client *redis.Client
}
func (s *RedisFingerrintStore) Get(ip string) (*Fingerprint, bool) {
    val, err := s.client.Get(ctx, "fp:"+ip).Bytes()
    // ...
}
```

2. **Design considerations:**
- Use Redis TTL for automatic expiry
- Consider BSON/MessagePack for smaller payloads
- Add circuit breaker for Redis failures
- Fallback to in-memory for resilience

The Redis store (`internal/store/store.go`) already exists but isn't used for fingerprints yet - this is a clear evolutionary path.
</details>

---

### Hard

**Q3.5:** The current middleware passes context values like this:
```go
ctx := context.WithValue(r.Context(), ja3ContextKey, ja3Fingerprint)
*r = *r.WithContext(ctx)
```
Why dereference and reassign the request? Is there a better approach?

<details>
<summary>Answer</summary>

**Current approach issues:**
- `*r = *r.WithContext(ctx)` modifies the original request pointer
- This is unusual and potentially confusing
- Works because `r` is a pointer to the request in stack

**Why it's done:**
The middleware doesn't call `next.ServeHTTP()` directly for unverified users, so the context value is being stored for use in `isSuspicious()` later in the same middleware.

**Better approaches:**

1. **Pass context to functions directly:**
```go
suspicious, score := isSuspiciousWithContext(ctx, r, loadedConfig)
```

2. **Use standard middleware pattern:**
```go
r = r.WithContext(ctx)
next.ServeHTTP(w, r)  // Pass modified request
```

3. **Store in request-scoped struct:**
```go
type requestData struct {
    JA3 string
    FP  *Fingerprint
}
ctx := context.WithValue(r.Context(), reqDataKey, &requestData{...})
```

The current code works but isn't idiomatic. Interview discussion point: "How would you refactor this?"
</details>

---

## 4. Concurrency & Thread Safety

### Easy

**Q4.1:** Why does the fingerprint store use `RWMutex` instead of regular `Mutex`?
```go
fingerprintStore.RLock()
fp, hasFingerprint := fingerprintStore.Data[clientIP]
fingerprintStore.RUnlock()
```

<details>
<summary>Answer</summary>

`RWMutex` allows multiple concurrent readers OR one exclusive writer:
- `RLock()` - Multiple goroutines can read simultaneously
- `Lock()` - Only one goroutine can write, blocks all readers

Since fingerprints are read far more often than written (every request reads, only challenge verification writes), `RWMutex` provides better concurrency than `Mutex` which would serialize all access.

Performance: With 1000 concurrent requests, `Mutex` processes serially. `RWMutex` processes 1000 reads in parallel.
</details>

---

**Q4.2:** Is this code thread-safe?
```go
challengeStore.RLock()
stored, exists := challengeStore.data[clientIP+req.Nonce]
challengeStore.RUnlock()

// Later...
if !challenge.VerifyChallenge(...) { return }

challengeStore.Lock()
delete(challengeStore.data, clientIP+req.Nonce)
challengeStore.Unlock()
```

<details>
<summary>Answer</summary>

**Potentially unsafe!** There's a race condition:
1. Goroutine A reads challenge (exists: true)
2. Goroutine B reads same challenge (exists: true)
3. Goroutine A verifies, deletes challenge
4. Goroutine B verifies same challenge, tries to delete (already gone)

**In this specific case:** It's mostly safe because:
- Delete on non-existent key is a no-op in Go maps
- The nonce is unique per challenge

**However, the real risk:** Replay attack during the verification window. Both requests could succeed before deletion.

**Fix:**
```go
challengeStore.Lock()
stored, exists := challengeStore.data[key]
if exists {
    delete(challengeStore.data, key)  // Delete immediately
}
challengeStore.Unlock()
if !exists { return error }
// Now verify with stored data
```
</details>

---

### Medium

**Q4.3:** The Redis rate limiter uses a pipeline:
```go
func (st *Store) IsRateLimited(identifier string, limit int) (bool, error) {
    key := "ratelimit:" + identifier
    pipe := st.rdb.Pipeline()
    count := pipe.Incr(ctx, key)
    pipe.ExpireNX(ctx, key, 1*time.Minute)
    _, err := pipe.Exec(ctx)
    return count.Val() > int64(limit), err
}
```
Why use a pipeline instead of separate commands? What's `ExpireNX`?

<details>
<summary>Answer</summary>

**Pipeline benefits:**
- Sends multiple commands in a single round-trip
- Reduces network latency from 2 RTTs to 1
- Commands execute atomically on Redis side
- More efficient for high-throughput systems

**ExpireNX:**
- "Expire if Not eXists" - Only sets expiry if key has no TTL
- Prevents resetting the 1-minute window on every request
- Without NX: First request sets 60s, second request resets to 60s (wrong!)
- With NX: First request sets 60s, subsequent requests don't touch it

**Race condition prevented:**
Without pipeline atomicity:
1. Request 1: INCR → count=1
2. Request 1: EXPIRE 60s
3. (59 seconds pass)
4. Request 2: INCR → count=2
5. Request 2: EXPIRE 60s  ← Reset! User gets another 60s

Pipeline ensures INCR + ExpireNX happen together.
</details>

---

**Q4.4:** What's wrong with this cleanup goroutine pattern?
```go
func init() {
    go func() {
        for {
            time.Sleep(1 * time.Minute)
            challengeStore.Lock()
            for key, stored := range challengeStore.data {
                if time.Now().After(stored.Expires) {
                    delete(challengeStore.data, key)
                }
            }
            challengeStore.Unlock()
        }
    }()
}
```

<details>
<summary>Answer</summary>

**Issues:**

1. **No graceful shutdown** - Goroutine runs forever, even during shutdown
```go
// Better:
func startCleanup(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(1 * time.Minute)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                cleanup()
            case <-ctx.Done():
                return
            }
        }
    }()
}
```

2. **Long lock hold time** - If map is large, holds lock for entire iteration
```go
// Better: Collect keys first, then delete
var toDelete []string
challengeStore.RLock()
for key, stored := range challengeStore.data {
    if time.Now().After(stored.Expires) {
        toDelete = append(toDelete, key)
    }
}
challengeStore.RUnlock()

challengeStore.Lock()
for _, key := range toDelete {
    delete(challengeStore.data, key)
}
challengeStore.Unlock()
```

3. **No error handling** - If something panics, goroutine dies silently
</details>

---

### Hard

**Q4.5:** Design a lock-free alternative for the fingerprint store using `sync.Map`. What are the trade-offs?

<details>
<summary>Answer</summary>

**Implementation:**
```go
type FingerprintStore struct {
    data sync.Map
}

func (s *FingerprintStore) Get(ip string) (Fingerprint, bool) {
    val, ok := s.data.Load(ip)
    if !ok {
        return Fingerprint{}, false
    }
    return val.(Fingerprint), true
}

func (s *FingerprintStore) Set(ip string, fp Fingerprint) {
    s.data.Store(ip, fp)
}
```

**Trade-offs:**

| Aspect | sync.Map | RWMutex + map |
|--------|----------|---------------|
| Read concurrency | ✅ Lock-free | ✅ Concurrent reads |
| Write concurrency | ✅ Lock-free | ❌ Exclusive lock |
| Read performance | ✅ Faster | ⚠️ Lock overhead |
| Write performance | ⚠️ Slower (copy-on-write) | ✅ Direct write |
| Memory usage | ❌ Higher | ✅ Lower |
| Range iteration | ⚠️ Callback-based | ✅ Simple for-range |
| Type safety | ❌ interface{} | ✅ Typed map |

**When to use sync.Map:**
- Read-heavy workloads (fingerprints fit this!)
- Keys are stable (not constantly added/removed)
- Different goroutines access different keys

**When to use RWMutex:**
- Need iteration over all entries
- Mixed read/write workloads
- Want type safety without reflection
</details>

---

## 5. Security Implementation

### Easy

**Q5.1:** The nonce is generated using:
```go
func generateNonce() (string, error) {
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return base64.StdEncoding.EncodeToString(b), nil
}
```
Why 16 bytes? Why `crypto/rand` instead of `math/rand`?

<details>
<summary>Answer</summary>

**16 bytes = 128 bits of entropy:**
- Sufficient to prevent brute-force (2^128 possibilities)
- Matches common security standards (UUID is 122 bits)
- Reasonable size for transmission (22 chars base64)

**crypto/rand vs math/rand:**
- `crypto/rand` uses OS entropy source (`/dev/urandom`, CryptGenRandom)
- `math/rand` is a PRNG - predictable if seed is known!
- Security-critical values MUST use `crypto/rand`

If attacker predicts nonce, they could:
1. Pre-compute proofs
2. Replay old challenges
3. Bypass verification entirely
</details>

---

**Q5.2:** Why does the JWT include the client IP?
```go
token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
    "ip":  clientIP,
    "exp": time.Now().Add(24 * time.Hour).Unix(),
})
```

<details>
<summary>Answer</summary>

**Token binding prevents token theft:**
1. Attacker steals `janus_token` cookie (XSS, network sniffing)
2. Attacker tries to use token from different IP
3. Verification checks: `claims["ip"] != currentIP` → Reject!

**Limitations:**
- Users behind NAT share IPs (could steal within network)
- Mobile users switch networks frequently (false rejections)
- VPN users may have issues

**Improvements:**
- Hash a fingerprint subset instead of IP
- Allow configurable strictness
- Use IP as one factor in confidence score
</details>

---

### Medium

**Q5.3:** The JA3 fingerprinting checks for "known browser" fingerprints:
```go
func isKnownBrowserJA3(ja3 string) bool {
    known := []string{
        "b2fa5d224d65e7c692fd46a0f52fce6b",
        // ...
    }
    for _, k := range known {
        if ja3 == k { return true }
    }
    return false
}
```
What are the security implications of this allowlist approach?

<details>
<summary>Answer</summary>

**Problems with allowlist:**

1. **Maintenance burden** - New browser versions change JA3
2. **False positives** - Legitimate browsers not in list get flagged
3. **Easy to bypass** - Bots can mimic known JA3 fingerprints
4. **Limited coverage** - Can't list all valid browsers

**Better approaches:**

1. **Blocklist known bots** instead:
```go
func isSuspiciousJA3(ja3 string) bool {
    badJA3 := []string{"...", "..."}  // Known scrapers
    return contains(badJA3, ja3)
}
```

2. **Consistency checking:**
```go
// If UA says Chrome, JA3 should match Chrome patterns
if isChrome(ua) && !isChromeLikeJA3(ja3) {
    score += 30  // Mismatch is suspicious
}
```

3. **ML-based classification** (future) - Train model on known good/bad JA3s
</details>

---

**Q5.4:** The canvas fingerprint verification compares hashes:
```go
if !isMobile && parts[5] != canvasHash {
    log.Printf("VerifyChallenge: Canvas hash mismatch")
    return false
}
```
Why would a legitimate user's canvas hash mismatch? How would you handle this?

<details>
<summary>Answer</summary>

**Legitimate mismatch causes:**
1. **Browser extensions** (privacy tools modify canvas)
2. **GPU driver updates** between fingerprint and verify
3. **Hardware changes** (docking station with different display)
4. **Browser anti-fingerprinting** (Safari, Firefox privacy mode)
5. **Font rendering changes** (system updates)
6. **Timing** - Canvas rendered differently due to load

**Handling strategies:**

1. **Fuzzy matching:**
```go
// Compare similarity instead of exact match
similarity := compareHashes(proofHash, storedHash)
if similarity < 0.95 { // Allow 5% variation
    score += 20  // Suspicious but not blocking
}
```

2. **Multiple fingerprints:**
```go
// Store array of recent hashes per IP
if contains(storedHashes, proofHash) { ok }
```

3. **Fallback challenges:**
```go
if canvasMismatch {
    // Don't reject, escalate to interactive challenge
    return Challenge{Type: "image"}
}
```

4. **Make it optional:**
```yaml
# config.yaml
strict_canvas_match: false
```
</details>

---

### Hard

**Q5.5:** Design a replay attack against this system and then design the defense.

<details>
<summary>Answer</summary>

**Attack scenario:**
1. Bot uses real browser to complete challenge once
2. Captures: fingerprint, challenge response, proof
3. Replays the exact same proof to `/janus/verify`
4. Gets `janus_token` cookie
5. Uses token for 24 hours of bot activity

**Current protections:**
- Nonce is deleted after use (single use)
- Timestamp must be within 5 minutes
- IP is baked into proof and token

**Vulnerabilities:**
- Race condition: Two requests before nonce deleted
- Token valid for 24hrs from any IP initially

**Defense enhancements:**

1. **Use-once nonce with Redis:**
```go
if !redisStore.ValidateNonce(nonce) {  // Atomic get+delete
    return errors.New("nonce already used")
}
```

2. **Shorter token lifetime:**
```go
"exp": time.Now().Add(1 * time.Hour).Unix(),
// Plus: sliding expiration on valid requests
```

3. **Request binding:**
```go
// Include more signals in token
claims := jwt.MapClaims{
    "ip":     clientIP,
    "canvas": hashOf(canvasFingerprint),
    "ua":     hashOf(userAgent),
    "exp":    ...,
}
```

4. **Continuous validation:**
```go
// On each request, check fingerprint consistency
currentFP := fingerprintStore.Get(ip)
if currentFP.Hash != claims["fpHash"] {
    // Fingerprint changed, re-challenge
}
```

5. **Rate limit successful verifications:**
```go
// Max 5 verifications per IP per hour
if verifyRateLimiter.Exceeded(ip) {
    return errors.New("too many verifications")
}
```
</details>

---

## 6. System Design

### Easy

**Q6.1:** Why does Janus use Redis for rate limiting instead of in-memory storage?

<details>
<summary>Answer</summary>

**Redis advantages:**
1. **Distributed** - Works across multiple instances
2. **Persistent** - Survives restarts
3. **Atomic operations** - INCR, EXPIRE are atomic
4. **TTL built-in** - Automatic cleanup

**Without Redis (multiple instances):**
- Instance A rate limits to 50/min
- Instance B rate limits to 50/min
- User hits both → 100 requests/min bypass!

**In-memory is fine for:**
- Single-instance deployments
- Development/testing
- Fallback if Redis fails

Current implementation correctly uses Redis for distributed rate limiting.
</details>

---

**Q6.2:** The system has two difficulty settings:
```yaml
desktop_difficulty: 8
mobile_difficulty: 6
```
Why different difficulties? How is this determined client-side?

<details>
<summary>Answer</summary>

**Why different:**
- Mobile CPUs are slower than desktop
- Mobile browsers have shorter script timeouts
- Battery considerations on mobile
- User experience expectations differ

**Difficulty 8 (desktop):** ~256 average SHA-256 iterations, ~50ms
**Difficulty 6 (mobile):** ~64 average iterations, ~20ms

**Client detection:**
```javascript
const isMobile = /Mobi|Android/i.test(navigator.userAgent);
```
Sent in fingerprint payload, server uses it when generating challenge.

**Potential improvement:** Use `navigator.hardwareConcurrency` and benchmark a quick hash to dynamically set difficulty.
</details>

---

### Medium

**Q6.3:** How would you scale Janus to handle 100,000 requests per second?

<details>
<summary>Answer</summary>

**Current bottlenecks:**
1. In-memory fingerprint store (single instance)
2. Challenge store (single instance)
3. GeoIP database loaded per instance

**Scaling strategy:**

1. **Stateless middleware:**
```go
// Move all state to Redis
- FingerprintStore → Redis (SET/GET with TTL)
- ChallengeStore → Redis (SET/GET with TTL)
- Rate limiting → Redis (already done)
```

2. **Horizontal scaling:**
```yaml
# Kubernetes HPA
minReplicas: 5
maxReplicas: 50
metrics:
  - type: Resource
    resource:
      name: cpu
      targetAverageUtilization: 70
```

3. **Redis cluster:**
```
- 3+ Redis nodes with sharding
- Use consistent hashing for key distribution
- Read replicas for fingerprint lookups
```

4. **CDN for static assets:**
```
- sensor.js served from CDN edge
- challenge.html served from CDN
```

5. **Load balancer configuration:**
```
- Sticky sessions not required (stateless)
- Health checks on /health endpoint
- Geographic load balancing
```

6. **Caching:**
```go
// Cache config, GeoIP results
var geoCache = lru.New(10000)
```

**Estimated capacity per instance:** 5,000 RPS
**For 100K RPS:** 20+ instances with headroom
</details>

---

**Q6.4:** Draw a sequence diagram for a distributed deployment where Redis is temporarily unavailable.

<details>
<summary>Answer</summary>

```
Client          Janus-1         Janus-2         Redis (DOWN)
  │                │               │                 │
  │ GET /page      │               │                 │
  │───────────────>│               │                 │
  │                │ IsRateLimited │                 │
  │                │──────────────────────────────>✗ │ (timeout)
  │                │               │                 │
  │                │ [Fallback to in-memory]          │
  │                │ Check local counter              │
  │                │               │                 │
  │                │ Get Fingerprint                  │
  │                │──────────────────────────────>✗ │
  │                │               │                 │
  │                │ [Fallback: require fresh FP]    │
  │ Challenge page │               │                 │
  │<───────────────│               │                 │
  │                │               │                 │
  │ POST /verify   │               │                 │
  │───────────────────────────────>│                 │
  │                │               │ IsRateLimited   │
  │                │               │────────────>✗   │
  │                │               │                 │
  │                │               │ [Degraded mode] │
  │  JWT cookie    │               │                 │
  │<───────────────────────────────│                 │
```

**Recommended fallback strategy:**
```go
func (st *Store) IsRateLimited(ip string, limit int) (bool, error) {
    limited, err := st.redisRateLimit(ip, limit)
    if err != nil {
        log.Warn("Redis unavailable, using local fallback")
        return st.localRateLimit(ip, limit), nil
    }
    return limited, nil
}
```

**Trade-offs in degraded mode:**
- ✅ Service remains available
- ⚠️ Rate limits not shared across instances
- ⚠️ Fingerprints not persisted
- Decision: Availability > Perfect security
</details>

---

### Hard

**Q6.5:** The current system uses Proof-of-Work. Design an alternative "Proof-of-Humanity" system that's more accessible but equally secure.

<details>
<summary>Answer</summary>

**Problems with PoW:**
- CPU-intensive (bad for mobile, accessibility)
- Can be GPU-accelerated by attackers
- Binary pass/fail (no gradual trust)

**Alternative: Behavioral Biometrics**

```
┌─────────────────────────────────────────────────────────────┐
│                    PROOF OF HUMANITY v2                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Phase 1: Passive Collection (invisible)                    │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ • Mouse velocity curves                              │   │
│  │ • Scroll acceleration patterns                       │   │
│  │ • Click timing variance                              │   │
│  │ • Touch pressure distribution (mobile)               │   │
│  │ • Accelerometer noise signature (mobile)             │   │
│  └─────────────────────────────────────────────────────┘   │
│                           ↓                                 │
│  Phase 2: Scoring (server-side)                            │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ entropy_score = calculate_entropy(behavior_data)     │   │
│  │                                                       │   │
│  │ if entropy_score > 0.8:                              │   │
│  │     → Issue token (high confidence)                  │   │
│  │ elif entropy_score > 0.5:                            │   │
│  │     → Simple interaction challenge                    │   │
│  │ else:                                                 │   │
│  │     → Full challenge (image puzzle)                  │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Interaction challenges (accessible):**
```javascript
// Option 1: Natural language
"Type 'I am human' in the box below"  // Keyboard dynamics analyzed

// Option 2: Simple gesture (mobile)
"Swipe the slider to unlock"  // Acceleration pattern analyzed

// Option 3: Audio challenge (accessibility)
"Press space when you hear 'cat'"  // Reaction time analyzed
```

**Implementation:**
```go
type BehaviorData struct {
    MouseMoves   []Movement  // {x, y, t, velocity}
    Scrolls      []Scroll    // {deltaY, t, acceleration}
    Clicks       []Click     // {x, y, t, pressure}
    KeyTimings   []KeyTiming // {key, downTime, upTime}
}

func CalculateHumanityScore(data BehaviorData) float64 {
    score := 0.0
    
    // Mouse jitter (humans have micro-tremors)
    score += analyzeMouseJitter(data.MouseMoves) * 0.2
    
    // Scroll acceleration curves (humans are smooth)
    score += analyzeScrollPatterns(data.Scrolls) * 0.2
    
    // Click timing variance (humans vary)
    score += analyzeClickTiming(data.Clicks) * 0.2
    
    // Typing rhythm (humans have patterns)
    score += analyzeKeyDynamics(data.KeyTimings) * 0.2
    
    // Statistical tests for randomness
    score += statisticalHumanness(data) * 0.2
    
    return score
}
```

**Benefits vs PoW:**
- ✅ Zero computational cost for users
- ✅ Invisible to legitimate users
- ✅ Progressive confidence (not binary)
- ✅ Accessible (works with assistive tech)
- ✅ Hard to fake (behavioral patterns)
- ⚠️ Requires ML model training
- ⚠️ Privacy considerations
</details>

---

## 7. Code Review Questions

**Q7.1:** Review this code and identify issues:
```go
func handleVerify(w http.ResponseWriter, r *http.Request) {
    clientIP := getClientIP(r)
    var req struct {
        Nonce string `json:"nonce"`
        Proof string `json:"proof"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    fingerprintStore.RLock()
    fp := fingerprintStore.Data[clientIP]
    fingerprintStore.RUnlock()
    
    challengeStore.RLock()
    stored := challengeStore.data[clientIP+req.Nonce]
    challengeStore.RUnlock()
    
    if !challenge.VerifyChallenge(req.Proof, req.Nonce, clientIP, 
        stored.Challenge.Seed, fp.IsMobile, fp.CanvasHash, loadedConfig) {
        http.Error(w, "Verification failed", 401)
        return
    }
    
    // Issue token...
}
```

<details>
<summary>Answer</summary>

**Issues identified:**

1. **No error handling on JSON decode:**
```go
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    http.Error(w, "Invalid JSON", http.StatusBadRequest)
    return
}
```

2. **No check if fingerprint exists:**
```go
fp, exists := fingerprintStore.Data[clientIP]
if !exists {
    http.Error(w, "No fingerprint", http.StatusBadRequest)
    return
}
```

3. **No check if challenge exists:**
```go
stored, exists := challengeStore.data[clientIP+req.Nonce]
if !exists {
    http.Error(w, "Invalid challenge", http.StatusBadRequest)
    return
}
```

4. **No expiry check on challenge:**
```go
if time.Now().After(stored.Expires) {
    http.Error(w, "Challenge expired", http.StatusBadRequest)
    return
}
```

5. **Potential nil pointer dereference:**
`stored.Challenge.Seed` would panic if `stored.Challenge` is nil

6. **Challenge not deleted after use** (allows replay)

7. **HTTP status code:** Use `http.StatusUnauthorized` instead of magic number `401`
</details>

---

**Q7.2:** This function logs sensitive data. What's wrong and how would you fix it?
```go
log.Printf("handleVerify: Proof verification failed for IP %s, nonce %s, proof %s", 
    clientIP, req.Nonce, req.Proof)
```

<details>
<summary>Answer</summary>

**Issues:**
1. **Full proof logged** - May contain sensitive canvas data
2. **Client IP in plain text** - PII concern (GDPR)
3. **Nonce logged** - Could aid replay attack analysis

**Fixed version:**
```go
log.Printf("handleVerify: failed ip_hash=%s nonce_prefix=%s", 
    hashIP(clientIP),
    req.Nonce[:8]+"...",
)

func hashIP(ip string) string {
    h := sha256.Sum256([]byte(ip + salt))
    return hex.EncodeToString(h[:8])  // First 8 bytes only
}
```

**Production logging best practices:**
- Hash PII before logging
- Truncate long values
- Use structured logging (JSON)
- Log to secure audit system
- Set appropriate log levels
</details>

---

## 8. Scenario-Based Questions

**Q8.1:** A customer reports that mobile users are failing verification 80% of the time. How would you debug this?

<details>
<summary>Answer</summary>

**Debugging steps:**

1. **Check logs for patterns:**
```bash
grep "mobile" janus.log | grep "failed" | head -100
# Look for common error messages
```

2. **Verify mobile detection:**
```go
// Potential bug: isMobile might be false when it should be true
log.Printf("UA: %s, isMobile: %v", ua, fp.IsMobile)
```

3. **Check difficulty settings:**
```yaml
mobile_difficulty: 6  # Is this too high?
mobile_iterations: 5000  # Is this timing out?
```

4. **Client-side timing:**
```javascript
// Add timing logs
const start = Date.now();
// ... PoW loop
console.log(`PoW took ${Date.now() - start}ms`);
// Mobile browsers may kill scripts after 10s
```

5. **Canvas hash on mobile:**
```javascript
// Mobile Safari/Chrome may have different canvas behavior
// Check if canvas_hash is consistent
```

6. **Network issues:**
```
// Mobile networks drop requests
// Check for partial fingerprint submission
```

**Common mobile failures:**
- Script timeout (iterations too high)
- Canvas rendering differences
- Touch events not captured
- Battery saver mode disables JS

**Quick fix while debugging:**
```yaml
mobile_difficulty: 0  # Disable PoW for mobile temporarily
```
</details>

---

**Q8.2:** Security team reports a bot is bypassing Janus by solving challenges once per IP and reusing tokens. Design a solution.

<details>
<summary>Answer</summary>

**Attack analysis:**
- Bot solves challenge from IP 1.2.3.4
- Gets valid `janus_token` JWT
- Uses token for bot requests until expiry
- Token is bound to IP, so one token per IP

**Why current defenses fail:**
- Token valid for 24 hours
- No continuous verification
- No behavioral analysis post-verification

**Multi-layer solution:**

1. **Short-lived tokens + refresh:**
```go
// Initial token valid 1 hour
"exp": time.Now().Add(1 * time.Hour).Unix()

// On each valid request, issue refresh
if tokenAge > 30*time.Minute {
    refreshToken(w, claims)
}
```

2. **Fingerprint must remain consistent:**
```go
func isVerified(r *http.Request) bool {
    // ... validate JWT
    
    // Check current fingerprint matches token
    currentFP := fingerprintStore.Get(ip)
    if currentFP.Hash != claims["fp_hash"] {
        return false  // Fingerprint changed, re-verify
    }
    return true
}
```

3. **Behavioral scoring post-verification:**
```go
type SessionMetrics struct {
    RequestCount   int
    UniquePages    int
    AvgRequestGap  time.Duration
    HasScrolled    bool
    HasMouseMoves  bool
}

func checkZombieIndicators(session *SessionMetrics) bool {
    // Bot pattern: many requests, no interaction
    if session.RequestCount > 10 && !session.HasScrolled {
        return true  // Suspicious
    }
    // Bot pattern: perfect timing between requests
    if session.AvgRequestGap == 1*time.Second { // Exactly 1s
        return true
    }
    return false
}
```

4. **Progressive re-verification:**
```go
if session.RequestCount % 20 == 0 {
    // Every 20 requests, require mini-challenge
    return Challenge{Type: "invisible-fingerprint-refresh"}
}
```

5. **Token fingerprint binding:**
```go
claims := jwt.MapClaims{
    "ip":         clientIP,
    "canvas_h":   hash(canvasFingerprint)[:16],
    "ua_h":       hash(userAgent)[:16],
    "created":    time.Now().Unix(),
    "req_count":  0,  // Track usage
}
```
</details>

---

**Q8.3:** You need to add WebSocket support to Janus for real-time challenge updates. How would you modify the architecture?

<details>
<summary>Answer</summary>

**Use case:**
- Progressive UI updates during challenge
- Server-initiated challenge escalation
- Real-time ban notifications

**Architecture changes:**

```
                     ┌─────────────────────────────────┐
                     │           Janus Server          │
                     │                                 │
 HTTP ──────────────►│ /janus/fingerprint (POST)       │
                     │ /janus/challenge   (GET)        │
                     │ /janus/verify      (POST)       │
                     │                                 │
 WebSocket ─────────►│ /janus/ws          (UPGRADE)   │
                     │   ├─ challengeUpdate            │
                     │   ├─ scoreUpdate                │
                     │   └─ verified                   │
                     └─────────────────────────────────┘
```

**Server implementation:**
```go
import "github.com/gorilla/websocket"

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true  // Configure properly in production
    },
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()
    
    clientIP := getClientIP(r)
    
    // Register connection
    wsClients.Store(clientIP, conn)
    defer wsClients.Delete(clientIP)
    
    for {
        _, message, err := conn.ReadMessage()
        if err != nil {
            break
        }
        handleWSMessage(clientIP, message)
    }
}

// Broadcast to client
func notifyClient(ip string, event WSEvent) {
    if conn, ok := wsClients.Load(ip); ok {
        conn.(*websocket.Conn).WriteJSON(event)
    }
}
```

**Client implementation:**
```javascript
const ws = new WebSocket('wss://localhost:8080/janus/ws');

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    switch(data.type) {
        case 'challengeUpdate':
            updateChallengeUI(data.difficulty);
            break;
        case 'scoreUpdate':
            updateProgressBar(data.score);
            break;
        case 'verified':
            window.location.href = '/';
            break;
    }
};
```

**Benefits:**
- Real-time feedback during challenge
- Server can adjust difficulty mid-challenge
- Immediate redirect on verification
- Reduced polling overhead

**Challenges:**
- WebSocket connection management
- Scaling (sticky sessions or Redis pub/sub)
- Graceful degradation to HTTP
</details>

---

## Bonus: Quick-Fire Questions

1. What's the difference between `http.Error()` and `w.WriteHeader()` + `w.Write()`?
2. Why use `chi` router instead of standard library `http.ServeMux`?
3. What does `defer` do and when is it executed?
4. Explain the difference between `make()` and `new()` in Go.
5. What's a goroutine leak and how do you prevent it?
6. Why is the JWT secret hardcoded and what's wrong with that?
7. What's the purpose of the `context` package?
8. How would you unit test the `isSuspicious()` function?
9. What's the N+1 problem and does this code have it?
10. How would you implement graceful shutdown for this server?

---

*Generated for Janus Project - Interview Preparation*
