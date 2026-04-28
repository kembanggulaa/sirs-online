package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLast_EfficientSeek(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Generate lines with predictable content
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("Line-%02d data here", i))
	}
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(logPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("gagal membuat file test log: %v", err)
	}

	tests := []struct {
		name      string
		readLines int
		expected  int
		wantFirst string // first line should contain this
		wantLast  string  // last line should contain this
	}{
		{"baca 5 baris terakhir", 5, 5, "Line-16", "Line-20"},
		{"baca 10 baris", 10, 10, "Line-11", "Line-20"},
		{"baca melebihi batas (30)", 30, 20, "Line-01", "Line-20"},
		{"baca 0 baris", 0, 0, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ReadLast(logPath, tt.readLines)
			if err != nil {
				t.Fatalf("error memanggil ReadLast: %v", err)
			}

			if len(result) != tt.expected {
				t.Errorf("ukuran result: expect %d, got %d", tt.expected, len(result))
			}

			if tt.expected > 0 {
				if !strings.Contains(result[0], tt.wantFirst) {
					t.Errorf("first line should contain %q, got %q", tt.wantFirst, result[0])
				}
				if !strings.Contains(result[len(result)-1], tt.wantLast) {
					t.Errorf("last line should contain %q, got %q", tt.wantLast, result[len(result)-1])
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
