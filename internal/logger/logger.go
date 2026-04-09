package logger

import (
	"bufio"
	"fmt"
	"io"
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

// ReadLast membaca n baris terakhir dari file log secara efisien
// menggunakan seek-from-end untuk menghindari membaca seluruh file ke memori.
func ReadLast(logPath string, n int) ([]string, error) {
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("gagal membuka file log: %w", err)
	}
	defer f.Close()

	// Dapatkan ukuran file
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("gagal stat file log: %w", err)
	}
	size := info.Size()
	if size == 0 {
		return []string{}, nil
	}

	// Baca file dari belakang, blok per blok, sampai kita punya cukup baris
	const chunkSize = 8192
	var buf []byte
	remaining := size

	for remaining > 0 {
		toRead := int64(chunkSize)
		if toRead > remaining {
			toRead = remaining
		}
		remaining -= toRead

		if _, err := f.Seek(remaining, io.SeekStart); err != nil {
			return nil, fmt.Errorf("seek gagal: %w", err)
		}

		chunk := make([]byte, toRead)
		if _, err := io.ReadFull(f, chunk); err != nil {
			return nil, fmt.Errorf("gagal baca chunk: %w", err)
		}
		buf = append(chunk, buf...)

		// Hitung berapa baris yang sudah kita punya
		lineCount := countLines(buf)
		if lineCount > n {
			break
		}
	}

	// Parse baris dari buffer yang sudah terkumpul
	lines := scanLines(buf)

	// Kembalikan n baris terakhir
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

// countLines menghitung jumlah baris dalam byte slice.
func countLines(b []byte) int {
	count := 0
	for _, c := range b {
		if c == '\n' {
			count++
		}
	}
	return count
}

// scanLines memisahkan byte slice menjadi slice of non-empty strings menggunakan bufio.Scanner.
// Menggantikan splitLines() O(n²) yang menggunakan string concatenation.
func scanLines(b []byte) []string {
	// Gunakan bufio.Scanner untuk O(n) parsing daripada string concatenation rune-by-rune
	type nopCloser struct{ *bufio.Scanner }

	scanner := bufio.NewScanner(newByteReader(b))
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// newByteReader membungkus []byte sebagai io.Reader untuk digunakan oleh bufio.Scanner.
type byteReader struct {
	data []byte
	pos  int
}

func newByteReader(b []byte) *byteReader {
	return &byteReader{data: b}
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
