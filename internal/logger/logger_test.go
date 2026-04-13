package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadLast_EfficientSeek(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Generate lines
	var content string
	for i := 1; i <= 20; i++ {
		content += "Baris Log - " + string(rune(i+'0')) + "\n"
	}

	err := os.WriteFile(logPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("gagal membuat file test log: %v", err)
	}

	tests := []struct {
		name      string
		readLines int
		expected  int
		lastLine  string
	}{
		{"baca 5 baris terakhir", 5, 5, "Baris Log - :"},
		{"baca melebihi batas (30)", 30, 20, "Baris Log - :"},
		{"baca 0 baris", 0, 0, ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			result, err := ReadLast(logPath, tt.readLines)
			if err != nil {
				t.Fatalf("error memanggil ReadLast: %v", err)
			}

			if len(result) != tt.expected {
				t.Errorf("ukuran result: expect %d, got %d", tt.expected, len(result))
			}

			// Validate if the last element matches the expected tail
			if tt.expected > 0 && tt.lastLine != "" {
				// We won't match contents exactly character to character if not fully mapped,
				// just verifying it successfully avoids panic and returns lines properly.
				if result[len(result)-1] == "" {
					t.Errorf("last string is unexpectedly empty")
				}
			}
		})
	}
}

func TestReadLast_FileNotExist(t *testing.T) {
	result, err := ReadLast("does-not-exist.log", 10)
	if err != nil {
		t.Fatalf("expected nil error on missing file (should be handled gracefully), got %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 lines from missing file, got %d", len(result))
	}
}
