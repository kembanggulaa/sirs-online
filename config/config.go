package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Config menyimpan semua konfigurasi aplikasi dari .env
type Config struct {
	// Database
	DBHost string
	DBPort int
	DBUser string
	DBPass string
	DBName string

	// API RS Online Kemenkes
	APIURL   string
	APIRsID  string
	APIPass  string

	// Operasional
	AppPort            int
	SyncIntervalHours  int
	RetryMax           int
	LogFile            string
	ExecutiveAPIURL    string
	TLSSkipVerify      bool
}

// Load membaca konfigurasi dari file .env menggunakan Viper
func Load() *Config {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")

	// Fallback ke environment variable
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("[WARN] Tidak bisa membaca .env: %v — menggunakan environment variable", err)
	}

	return &Config{
		DBHost:            viper.GetString("DB_HOST"),
		DBPort:            viper.GetInt("DB_PORT"),
		DBUser:            viper.GetString("DB_USER"),
		DBPass:            viper.GetString("DB_PASS"),
		DBName:            viper.GetString("DB_NAME"),
		APIURL:            viper.GetString("API_URL"),
		APIRsID:           viper.GetString("API_RS_ID"),
		APIPass:           viper.GetString("API_PASS"),
		AppPort:           viper.GetInt("APP_PORT"),
		SyncIntervalHours: viper.GetInt("SYNC_INTERVAL_HOURS"),
		RetryMax:          viper.GetInt("RETRY_MAX"),
		LogFile:           viper.GetString("LOG_FILE"),
		ExecutiveAPIURL:   viper.GetString("EXECUTIVE_API_URL"),
		TLSSkipVerify:     viper.GetBool("TLS_SKIP_VERIFY"),
	}
}
