package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

// DatabaseConfig menyimpan konfigurasi koneksi ke database
type DatabaseConfig struct {
	Host string
	Port int
	User string
	Pass string
	Name string
}

// APIConfig menyimpan konfigurasi API Kemenkes dan endpoint eksternal lainnya
type APIConfig struct {
	URL          string
	RsID         string
	Pass         string
	ExecutiveURL string // URL API Dashboard Eksekutif
}

// OperationalConfig menyimpan konfigurasi operasional server dan worker
type OperationalConfig struct {
	AppPort           int
	SyncIntervalHours int
	RetryMax          int
	LogFile           string
	TLSSkipVerify     bool
	OrgUnitCode       string
}

// SecurityConfig menyimpan konfigurasi sistem keamanan seperti CORS dan request body
type SecurityConfig struct {
	DashboardOrigin string // asal dashboard untuk header CORS (misal: http://localhost:9271)
	MaxBodyBytes    int64  // batas ukuran request body POST/PUT (default: 1 MB)
}

// Config menyimpan semua konfigurasi aplikasi dari .env
type Config struct {
	Database    DatabaseConfig
	API         APIConfig
	Operational OperationalConfig
	Security    SecurityConfig
}

// Load membaca konfigurasi dari file .env menggunakan Viper.
// Fungsi ini menggunakan global state Viper dan tidak aman untuk dipanggil secara concurrent.
// Panggil sekali saja di awal startup sebelum goroutine lain dimulai.
func Load() *Config {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")

	// Fallback ke environment variable
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("[WARN] Tidak bisa membaca .env: %v \u2014 menggunakan environment variable", err)
	}

	return &Config{
		Database: DatabaseConfig{
			Host: viper.GetString("DB_HOST"),
			Port: viper.GetInt("DB_PORT"),
			User: viper.GetString("DB_USER"),
			Pass: viper.GetString("DB_PASS"),
			Name: viper.GetString("DB_NAME"),
		},
		API: APIConfig{
			URL:          viper.GetString("API_URL"),
			RsID:         viper.GetString("API_RS_ID"),
			Pass:         viper.GetString("API_PASS"),
			ExecutiveURL: viper.GetString("EXECUTIVE_API_URL"),
		},
		Operational: OperationalConfig{
			AppPort:           viper.GetInt("APP_PORT"),
			SyncIntervalHours: viper.GetInt("SYNC_INTERVAL_HOURS"),
			RetryMax:          viper.GetInt("RETRY_MAX"),
			LogFile:           viper.GetString("LOG_FILE"),
			TLSSkipVerify:     viper.GetBool("TLS_SKIP_VERIFY"),
			OrgUnitCode:       viper.GetString("ORG_UNIT_CODE"),
		},
		Security: SecurityConfig{
			DashboardOrigin: viper.GetString("DASHBOARD_ORIGIN"),
			MaxBodyBytes:    viper.GetInt64("MAX_BODY_BYTES"),
		},
	}
}
