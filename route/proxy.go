package route

import (
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	gometrics "github.com/eBay/fabio/_third_party/github.com/rcrowley/go-metrics"
	"github.com/eBay/fabio/config"
)

// Proxy is a dynamic reverse proxy.
type Proxy struct {
	tr       http.RoundTripper
	cfg      config.Proxy
	requests gometrics.Timer
}

func NewProxy(tr http.RoundTripper, cfg config.Proxy) *Proxy {
	return &Proxy{
		tr:       tr,
		cfg:      cfg,
		requests: gometrics.GetOrRegisterTimer("requests", gometrics.DefaultRegistry),
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if ShuttingDown() {
		http.Error(w, "shutting down", http.StatusServiceUnavailable)
		return
	}

	target := GetTable().lookup(req, req.Header.Get("trace"))
	if target == nil {
		log.Print("[WARN] No route for ", req.URL)
		w.WriteHeader(404)
		return
	}

	if err := addHeaders(req, p.cfg); err != nil {
		http.Error(w, "cannot parse "+req.RemoteAddr, http.StatusInternalServerError)
		return
	}

	start := time.Now()
	rp := httputil.NewSingleHostReverseProxy(target.URL)
	rp.Transport = p.tr
	rp.ServeHTTP(w, req)
	target.timer.UpdateSince(start)
	p.requests.UpdateSince(start)
}

func addHeaders(r *http.Request, cfg config.Proxy) error {
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return errors.New("cannot parse " + r.RemoteAddr)
	}

	if cfg.ClientIPHeader != "" {
		r.Header.Set(cfg.ClientIPHeader, remoteIP)
	}

	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" && cfg.LocalIP != "" {
		r.Header.Set("X-Forwarded-For", xff+", "+cfg.LocalIP)
	}

	fwd := r.Header.Get("Forwarded")
	if fwd == "" {
		fwd = "for=" + remoteIP
		if r.TLS != nil {
			fwd += "; proto=https"
		} else {
			fwd += "; proto=http"
		}
	}
	if cfg.LocalIP != "" {
		fwd += "; by=" + cfg.LocalIP
	}
	r.Header.Set("Forwarded", fwd)

	if cfg.TLSHeader != "" && r.TLS != nil {
		r.Header.Set(cfg.TLSHeader, cfg.TLSHeaderValue)
	}

	return nil
}
