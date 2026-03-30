package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
	_ "github.com/microsoft/go-mssqldb"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"

	"sirs-online/config"
	"sirs-online/internal/handler"
	"sirs-online/internal/logger"
	"sirs-online/internal/repository"
	"sirs-online/internal/worker"
)

// ─── Windows Service ──────────────────────────────────────────────────────────

type sirsService struct{}

func (s *sirsService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	go run()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for {
		c := <-r
		switch c.Cmd {
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			return false, 0
		}
	}
}

// ─── Main Entry Point ─────────────────────────────────────────────────────────

func main() {
	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		log.Fatalf("Gagal memeriksa mode sesi: %v", err)
	}

	if isInteractive {
		// Mode development — jalankan langsung tanpa Windows Service wrapper
		log.Println("Mode interaktif — menjalankan sebagai console app")
		run()
	} else {
		// Mode produksi — jalankan sebagai Windows Service
		if err := debug.Run("sirs-online", &sirsService{}); err != nil {
			log.Fatalf("Gagal menjalankan Windows Service: %v", err)
		}
	}
}

// run berisi logika utama aplikasi
func run() {
	// 1. Load konfigurasi
	cfg := config.Load()

	// 2. Init logger
	if err := logger.Init(cfg.LogFile); err != nil {
		log.Fatalf("Gagal inisialisasi logger: %v", err)
	}
	defer logger.Close()

	logger.Info("=== SIRS Online Bridging V3 start ===")
	logger.Info("PORT=%d | INTERVAL=%dj | LOG=%s", cfg.AppPort, cfg.SyncIntervalHours, cfg.LogFile)

	// 3. Koneksi ke SQL Server SIMRS
	dsn := fmt.Sprintf(
		"server=%s;port=%d;user id=%s;password=%s;database=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPass, cfg.DBName,
	)
	db, err := sql.Open("mssql", dsn)
	if err != nil {
		logger.Error("Gagal membuka koneksi DB: %v", err)
		log.Fatalf("DB open error: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		logger.Error("Tidak bisa terhubung ke SQL Server: %v", err)
		log.Fatalf("DB ping error: %v", err)
	}
	logger.Info("Koneksi SQL Server berhasil: %s:%d/%s", cfg.DBHost, cfg.DBPort, cfg.DBName)

	// 4. Inisialisasi repository & dispatcher
	repo := repository.New(db)
	skRepo := repository.NewSKRepository(db)
	dispatcher := worker.NewDispatcher(repo, cfg)

	// 5. Mulai Ticker (berjalan di background)
	go dispatcher.Start()

	// 6. Setup HTTP Server
	mux := http.NewServeMux()

	// Endpoint internal
	apiHandler := handler.New(cfg, dispatcher)
	apiHandler.RegisterRoutes(mux)

	skHandler := handler.NewSKHandler(skRepo)
	skHandler.RegisterRoutes(mux)

	// Endpoint proxy Kemenkes — Tab 2: GET referensi TT dari Kemenkes
	mux.HandleFunc("GET /api/proxy/referensi", makeProxyHandler(cfg, "GET",
		cfg.APIURL+"/Referensi/tempat_tidur"))

	// Endpoint proxy Kemenkes — Tab 3: GET data Fasyankes yang sudah diinputkan RS
	mux.HandleFunc("GET /api/proxy/fasyankes", makeProxyHandler(cfg, "GET",
		cfg.APIURL+"/Fasyankes"))

	// Endpoint proxy Kemenkes — Tab 4: POST tempat tidur baru
	mux.HandleFunc("POST /api/kemenkes/tempat-tidur", makeKemenkesForwardHandler(cfg, "POST",
		cfg.APIURL+"/Fasyankes"))

	// Endpoint proxy Kemenkes — Tab 4: PUT tempat tidur (update manual)
	mux.HandleFunc("PUT /api/kemenkes/tempat-tidur/{id_tt}", func(w http.ResponseWriter, r *http.Request) {
		makeKemenkesForwardHandler(cfg, "PUT", cfg.APIURL+`/Fasyankes`)(w, r)
	})

	// Endpoint Eksekutif — Dashboard Khusus (Data total per bangsal)
	mux.HandleFunc("GET /api/beds/executive", makeProxyHandler(cfg, "GET",
		cfg.ExecutiveAPIURL))

	addr := ":" + strconv.Itoa(cfg.AppPort)
	logger.Info("Dashboard berjalan di http://localhost%s", addr)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil {
		logger.Error("HTTP server error: %v", err)
		log.Fatal(err)
	}
}

// ─── Proxy Helpers ────────────────────────────────────────────────────────────

// makeProxyHandler membuat handler GET read-only ke Kemenkes (untuk Tab 2)
func makeProxyHandler(cfg *config.Config, method, url string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client := resty.New().SetTimeout(15 * time.Second)
		timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)

		resp, err := client.R().
			SetHeader("X-rs-id", cfg.APIRsID).
			SetHeader("X-pass", cfg.APIPass).
			SetHeader("X-Timestamp", timestamp).
			Execute(method, url)

		if err != nil {
			http.Error(w, "Gagal menghubungi API Kemenkes: "+err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(resp.StatusCode())
		_, _ = w.Write(resp.Body())
	}
}

// makeKemenkesForwardHandler meneruskan request POST/PUT dari dashboard ke Kemenkes
func makeKemenkesForwardHandler(cfg *config.Config, method, url string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Body tidak valid: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer func() { _ = r.Body.Close() }()

		client := resty.New().SetTimeout(15 * time.Second)
		timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)

		resp, err := client.R().
			SetHeader("X-rs-id", cfg.APIRsID).
			SetHeader("X-pass", cfg.APIPass).
			SetHeader("X-Timestamp", timestamp).
			SetHeader("Content-Type", "application/json").
			SetBody(body).
			Execute(method, url)

		if err != nil {
			http.Error(w, "Gagal menghubungi API Kemenkes: "+err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(resp.StatusCode())
		_, _ = w.Write(resp.Body())
	}
}
