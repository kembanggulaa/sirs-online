package worker

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

// NewKemenkesClient membuat resty client yang dikonfigurasi untuk API Kemenkes.
// skipTLSVerify: true untuk mengabaikan verifikasi sertifikat SSL
// (umum diperlukan untuk API pemerintah Indonesia).
func NewKemenkesClient(skipTLSVerify bool) *resty.Client {
	transport := &http.Transport{
		DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLSVerify,
		},
	}
	return resty.New().
		SetTimeout(30 * time.Second).
		SetTransport(transport)
}
