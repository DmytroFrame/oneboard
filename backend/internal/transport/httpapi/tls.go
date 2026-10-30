package httpapi

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// certReloader serves the certificate from disk and reloads it when the files change, so a renewal
// (e.g. certbot every ~60 days) takes effect without restarting the server.
type certReloader struct {
	certFile, keyFile string
	log               *slog.Logger

	mu        sync.Mutex
	cert      *tls.Certificate
	modTime   time.Time
	checkedAt time.Time
}

const certCheckInterval = time.Minute

func newCertReloader(certFile, keyFile string, log *slog.Logger) (*certReloader, error) {
	r := &certReloader{certFile: certFile, keyFile: keyFile, log: log}
	if err := r.load(); err != nil {
		return nil, err // a broken certificate at start is a configuration error
	}
	return r, nil
}

func (r *certReloader) load() error {
	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		return fmt.Errorf("load TLS certificate: %w", err)
	}
	r.cert = &cert
	r.modTime = r.lastModified()
	if cert.Leaf != nil {
		r.log.Info("TLS certificate loaded", "subject", cert.Leaf.Subject.CommonName, "expires", cert.Leaf.NotAfter)
	}
	return nil
}

// lastModified follows symlinks (Let's Encrypt's live/ files point into archive/).
func (r *certReloader) lastModified() time.Time {
	var latest time.Time
	for _, f := range []string{r.certFile, r.keyFile} {
		if info, err := os.Stat(f); err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest
}

func (r *certReloader) get(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if time.Since(r.checkedAt) > certCheckInterval {
		r.checkedAt = time.Now()
		if r.lastModified().After(r.modTime) {
			// Keep serving the old certificate if the new files are half-written or broken.
			if err := r.load(); err != nil {
				r.log.Error("reload TLS certificate", "err", err)
			}
		}
	}
	return r.cert, nil
}

// redirectToHTTPS answers plain HTTP with a permanent redirect to the same URL over HTTPS.
// ACME HTTP-01 challenges (certbot --webroot) are served from webroot instead, so certificates
// can be renewed while the server keeps port 80.
func redirectToHTTPS(httpsAddr, webroot string) http.Handler {
	_, port, _ := net.SplitHostPort(httpsAddr)
	var challenges http.Handler
	if webroot != "" {
		challenges = http.FileServer(http.Dir(filepath.Clean(webroot)))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if challenges != nil && strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
			challenges.ServeHTTP(w, r)
			return
		}
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if port != "" && port != "443" {
			host = net.JoinHostPort(host, port)
		}
		// 308 keeps the method and body (a 301 would turn a POST into a GET).
		http.Redirect(w, r, "https://"+host+r.URL.RequestURI(), http.StatusPermanentRedirect)
	})
}
