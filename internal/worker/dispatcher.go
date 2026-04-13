package worker

import (
	"context"
	"time"

	"sirs-online/config"
	"sirs-online/internal/logger"
	"sirs-online/internal/repository"
)

// Job membawa semua konteks yang dibutuhkan satu siklus sync
type Job struct {
	Repo   repository.BedRepositoryInterface
	Config *config.Config
}

// Dispatcher mengatur penjadwalan dan pengiriman Job ke Worker
type Dispatcher struct {
	jobQueue chan Job
	done     chan struct{}
	repo     repository.BedRepositoryInterface
	cfg      *config.Config
}

// NewDispatcher membuat Dispatcher baru dan memulai listener worker
func NewDispatcher(repo repository.BedRepositoryInterface, cfg *config.Config) *Dispatcher {
	d := &Dispatcher{
		jobQueue: make(chan Job, 1),
		done:     make(chan struct{}),
		repo:     repo,
		cfg:      cfg,
	}
	// Jalankan satu worker goroutine permanen
	go runWorker(d.jobQueue)
	return d
}


// StartWithContext memulai Ticker otomatis dan menghentikannya saat ctx selesai.
// Harus dipanggil di goroutine terpisah.
func (d *Dispatcher) StartWithContext(ctx context.Context) {
	interval := time.Duration(d.cfg.Operational.SyncIntervalHours) * time.Hour
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info("Dispatcher dimulai. Interval: %v", interval)

	for {
		select {
		case <-ticker.C:
			d.dispatch()
		case <-d.done:
			logger.Info("Dispatcher dihentikan.")
			return
		case <-ctx.Done():
			logger.Info("Dispatcher dihentikan (context cancelled).")
			return
		}
	}
}

// TriggerManual mengirim Job secara manual (dari tombol Sync Now di dashboard).
// Mengembalikan false jika worker sedang berjalan (skip).
func (d *Dispatcher) TriggerManual() bool {
	return d.dispatch()
}

// IsRunning memeriksa apakah worker sedang aktif memproses job.
// Membaca dari runningFlag package-level yang di-update oleh worker.go.
func (d *Dispatcher) IsRunning() bool {
	return IsWorkerRunning()
}

// Stop menghentikan Dispatcher
func (d *Dispatcher) Stop() {
	close(d.done)
}

// dispatch mencoba mengirim job baru ke worker.
// Jika worker masih berjalan, job di-skip (tidak menumpuk).
func (d *Dispatcher) dispatch() bool {
	if IsWorkerRunning() {
		logger.Info("Worker masih berjalan — tick di-skip")
		return false
	}

	job := Job{
		Repo:   d.repo,
		Config: d.cfg,
	}

	select {
	case d.jobQueue <- job:
		return true
	default:
		// Queue penuh (sangat jarang karena buffer 1) — skip
		logger.Info("Job queue penuh — tick di-skip")
		return false
	}
}
