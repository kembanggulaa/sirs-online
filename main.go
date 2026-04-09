package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go run(ctx)

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for {
		c := <-r
		switch c.Cmd {
		case svc.Stop, svc.Shutdown:
			cancel()
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

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		run(ctx)
	} else {
		// Mode produksi — jalankan sebagai Windows Service
		if err := debug.Run("sirs-online", &sirsService{}); err != nil {
			log.Fatalf("Gagal menjalankan Windows Service: %v", err)
		}
	}
}

// run berisi logika utama aplikasi. Menerima context untuk graceful shutdown.
func run(ctx context.Context) {
	// 1. Load konfigurasi
	cfg := config.Load()

	// 2. Init logger
	if err := logger.Init(cfg.LogFile); err != nil {
		log.Fatalf("Gagal inisialisasi logger: %v", err)
	}
	defer logger.Close()

	logger.Info("=== SIRS Online Bridging V3 start ===")
	logger.Info("PORT=%d | INTERVAL=%dj | LOG=%s | TLS_SKIP_VERIFY=%v",
		cfg.AppPort, cfg.SyncIntervalHours, cfg.LogFile, cfg.TLSSkipVerify)

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
	bedsRepo := repository.NewBedsRepository(db, cfg.OrgUnitCode)
	dispatcher := worker.NewDispatcher(repo, cfg)

	// 5. Mulai Ticker (berjalan di background, dihentikan saat ctx selesai)
	go dispatcher.StartWithContext(ctx)

	// 6. Setup HTTP Server
	mux := http.NewServeMux()

	// Endpoint internal
	apiHandler := handler.New(cfg, dispatcher)
	apiHandler.RegisterRoutes(mux)

	skHandler := handler.NewSKHandler(skRepo, cfg)
	skHandler.RegisterRoutes(mux)

	bedsHandler := handler.NewBedsHandler(bedsRepo, cfg)
	bedsHandler.RegisterRoutes(mux)

	// Endpoint proxy Kemenkes (dikelola oleh ProxyHandler)
	proxyHandler := handler.NewProxyHandler(cfg)
	proxyHandler.RegisterRoutes(mux)

	addr := fmt.Sprintf(":%d", cfg.AppPort)
	logger.Info("Dashboard berjalan di http://localhost%s", addr)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 7. Jalankan HTTP server di goroutine, shutdown gracefully saat ctx selesai
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("Shutdown signal diterima — menghentikan server...")
		dispatcher.Stop()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP shutdown error: %v", err)
		} else {
			logger.Info("Server berhenti dengan baik.")
		}

	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error: %v", err)
			log.Fatal(err)
		}
	}
}
