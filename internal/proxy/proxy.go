package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"janus/internal/config"
	"janus/internal/store"
)

type Proxy struct {
	rp  *httputil.ReverseProxy
	cfg *config.Config
}

func NewProxy(cfg *config.Config) (http.Handler, error) {
	u, err := url.Parse(cfg.Backend)
	if err != nil {
		return nil, err
	}
	rp := httputil.NewSingleHostReverseProxy(u)

	// Preserve original director but ensure Host/Scheme
	origDirector := rp.Director
	rp.Director = func(req *http.Request) {
		origDirector(req)
		req.Header.Set("X-Janus-Proxy", "janus/1.0")
	}

	// Inject script into HTML responses (simple approach; skips compressed responses)
	rp.ModifyResponse = func(resp *http.Response) error {
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(strings.ToLower(ct), "text/html") {
			return nil
		}
		if resp.Header.Get("Content-Encoding") != "" {
			// skip compressed bodies in MVP
			return nil
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		resp.Body.Close()

		// generate nonce and store it against the client's IP
		nonce := store.GenerateNonce()
		if resp.Request != nil {
			store.AddNonce(resp.Request.RemoteAddr, nonce, cfg.NonceTTLSeconds)
		}

		// prepare script tag
		script := `<script src="` + cfg.InjectScriptPath + `" data-nonce="` + nonce + `"></script>`

		lower := strings.ToLower(string(body))
		var newBody []byte
		if idx := strings.Index(lower, "</head>"); idx != -1 {
			newBody = append(body[:idx], append([]byte(script), body[idx:]...)...)
		} else if idx := strings.Index(lower, "</body>"); idx != -1 {
			newBody = append(body[:idx], append([]byte(script), body[idx:]...)...)
		} else {
			newBody = append(body, []byte(script)...)
		}

		resp.Body = io.NopCloser(bytes.NewReader(newBody))
		resp.ContentLength = int64(len(newBody))
		resp.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
		resp.Header.Del("Content-Encoding")
		return nil
	}

	return rp, nil
}
