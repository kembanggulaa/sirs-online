package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var fileLogger *log.Logger
var logFile *os.File

// Init menginisialisasi logger file. Harus dipanggil sekali saat startup.
func Init(logPath string) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return fmt.Errorf("gagal membuat direktori log: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("gagal membuka file log: %w", err)
	}

	logFile = f
	fileLogger = log.New(f, "", 0)
	return nil
}

// Close menutup file log. Dipanggil saat aplikasi shutdown.
func Close() {
	if logFile != nil {
		_ = logFile.Close()
	}
}

func timestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// Info mencatat pesan informasi
func Info(format string, args ...interface{}) {
	msg := fmt.Sprintf("[INFO] "+format, args...)
	log.Println(msg)
	if fileLogger != nil {
		fileLogger.Printf("%s %s\n", timestamp(), msg)
	}
}

// Warn mencatat pesan peringatan
func Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf("[WARN] "+format, args...)
	log.Println(msg)
	if fileLogger != nil {
		fileLogger.Printf("%s %s\n", timestamp(), msg)
	}
}

// Error mencatat pesan error
func Error(format string, args ...interface{}) {
	msg := fmt.Sprintf("[ERROR] "+format, args...)
	log.Println(msg)
	if fileLogger != nil {
		fileLogger.Printf("%s %s\n", timestamp(), msg)
	}
}

// ReadLast membaca n baris terakhir dari file log
func ReadLast(logPath string, n int) ([]string, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	lines := splitLines(string(data))
	if len(lines) <= n {
		return lines, nil
	}
	return lines[len(lines)-n:], nil
}

func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, r := range s {
		if r == '\n' {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
