package config

import (
	"testing"
)

func TestLoad_EnvironmentVariables(t *testing.T) {
	// Setup enviroment variables to mock .env file configuration
	t.Setenv("DB_HOST", "test-host")
	t.Setenv("APP_PORT", "8888")
	t.Setenv("TLS_SKIP_VERIFY", "true")
	t.Setenv("MAX_BODY_BYTES", "2048")
	t.Setenv("API_URL", "https://test-api.example.com")
	t.Setenv("API_RS_ID", "RS-TEST-001")
	t.Setenv("DASHBOARD_ORIGIN", "http://localhost:3000")
	t.Setenv("EXECUTIVE_API_URL", "https://executive.example.com")

	cfg := Load()

	// Database sub-struct
	if cfg.Database.Host != "test-host" {
		t.Errorf("expected Database.Host to be 'test-host', got %q", cfg.Database.Host)
	}

	// Operational sub-struct
	if cfg.Operational.AppPort != 8888 {
		t.Errorf("expected Operational.AppPort to be 8888, got %d", cfg.Operational.AppPort)
	}
	if cfg.Operational.TLSSkipVerify != true {
		t.Error("expected Operational.TLSSkipVerify to be true")
	}

	// API sub-struct
	if cfg.API.URL != "https://test-api.example.com" {
		t.Errorf("expected API.URL to be 'https://test-api.example.com', got %q", cfg.API.URL)
	}
	if cfg.API.RsID != "RS-TEST-001" {
		t.Errorf("expected API.RsID to be 'RS-TEST-001', got %q", cfg.API.RsID)
	}
	if cfg.API.ExecutiveURL != "https://executive.example.com" {
		t.Errorf("expected API.ExecutiveURL to be 'https://executive.example.com', got %q", cfg.API.ExecutiveURL)
	}

	// Security sub-struct
	if cfg.Security.MaxBodyBytes != 2048 {
		t.Errorf("expected Security.MaxBodyBytes to be 2048, got %d", cfg.Security.MaxBodyBytes)
	}
	if cfg.Security.DashboardOrigin != "http://localhost:3000" {
		t.Errorf("expected Security.DashboardOrigin to be 'http://localhost:3000', got %q", cfg.Security.DashboardOrigin)
	}
}

func TestLoad_Defaults(t *testing.T) {
	// If a variable is missing and no default is explicitly set by us in code,
	// Viper sets it to the zero value of the type requested.
	// Ensure that clearing a var results in the zero value of that type.
	t.Setenv("DB_PORT", "") // override it with empty

	cfg := Load()

	if cfg.Database.Port != 0 {
		t.Errorf("expected DB_PORT to fallback to 0 when empty, got %d", cfg.Database.Port)
	}
}
