package worker

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

var (
	kemenkesClientOnce sync.Once
	kemenkesClient     *resty.Client
	kemenkesClientMu   sync.RWMutex
	kemenkesSkipTLS    bool
)

// NewKemenkesClient mengembalikan resty client yang dikonfigurasi untuk API Kemenkes.
// Client ini adalah singleton — dibuat sekali dan di-reuse untuk semua request
// sehingga TCP keep-alive dan connection pooling bekerja optimal.
//
// skipTLSVerify: true untuk mengabaikan verifikasi sertifikat SSL
// (umum diperlukan untuk API pemerintah Indonesia).
//
// Catatan: jika skipTLSVerify berubah antar-panggilan (sangat jarang),
// singleton akan dibuat ulang.
func NewKemenkesClient(skipTLSVerify bool) *resty.Client {
	kemenkesClientMu.RLock()
	existing := kemenkesClient
	sameConfig := (existing != nil && kemenkesSkipTLS == skipTLSVerify)
	kemenkesClientMu.RUnlock()

	if sameConfig {
		return existing
	}

	// Buat ulang client jika config berubah atau pertama kali
	kemenkesClientMu.Lock()
	defer kemenkesClientMu.Unlock()

	// Double-check after acquiring write lock
	if kemenkesClient != nil && kemenkesSkipTLS == skipTLSVerify {
		return kemenkesClient
	}

	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLSVerify, //nolint:gosec // diperlukan untuk API pemerintah
		},
	}

	kemenkesClient = resty.New().
		SetTimeout(30 * time.Second).
		SetTransport(transport)
	kemenkesSkipTLS = skipTLSVerify

	return kemenkesClient
}
