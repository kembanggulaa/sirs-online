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
	
	// Create an empty .env locally so viper can find it if needed (not strictly required if AutomaticEnv overrides it perfectly).
	// To prevent viper reading actual dev .env, we can just rely on viper overriding logic.
	
	cfg := Load()

	if cfg.DBHost != "test-host" {
		t.Errorf("expected DBHost to be 'test-host', got %q", cfg.DBHost)
	}

	if cfg.AppPort != 8888 {
		t.Errorf("expected AppPort to be 8888, got %d", cfg.AppPort)
	}

	if cfg.TLSSkipVerify != true {
		t.Error("expected TLSSkipVerify to be true")
	}

	if cfg.MaxBodyBytes != 2048 {
		t.Errorf("expected MaxBodyBytes to be 2048, got %d", cfg.MaxBodyBytes)
	}
}

func TestLoad_Defaults(t *testing.T) {
	// If a variable is missing and no default is explicitly set by us in code, 
	// Viper sets it to the zero value of the type requested.
	// Ensure that clearing a var results in the zero value of that type.
	t.Setenv("DB_PORT", "") // override it with empty
	
	cfg := Load()
	
	if cfg.DBPort != 0 {
		t.Errorf("expected DB_PORT to fallback to 0 when empty, got %d", cfg.DBPort)
	}
}
