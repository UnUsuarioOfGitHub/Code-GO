package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	APIHost             string
	APIPort             int
	RequestTimeout      time.Duration
	ReadHeaderTimeout   time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
	ShutdownTimeout     time.Duration
	DBHost              string
	DBPort              int
	DBUser              string
	DBPassword          string
	DBName              string
	DBMaxConns          int32
	DBMinConns          int32
	DBMaxConnLifetime   time.Duration
	DBMaxConnIdleTime   time.Duration
	DBHealthCheckPeriod time.Duration
	LogFormat           string
	LogLevel            string
}

func Load() (Config, error) {
	var cfg Config
	var err error

	if cfg.APIHost, err = stringValue("API_HOST", "0.0.0.0"); err != nil {
		return Config{}, err
	}
	if cfg.APIPort, err = intValue("API_PORT", 8080); err != nil {
		return Config{}, err
	}
	if cfg.RequestTimeout, err = durationValue("REQUEST_TIMEOUT", 5*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ReadHeaderTimeout, err = durationValue("HTTP_READ_HEADER_TIMEOUT", 5*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ReadTimeout, err = durationValue("HTTP_READ_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.WriteTimeout, err = durationValue("HTTP_WRITE_TIMEOUT", 15*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.IdleTimeout, err = durationValue("HTTP_IDLE_TIMEOUT", 60*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = durationValue("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.DBHost, err = stringValue("DB_HOST", "localhost"); err != nil {
		return Config{}, err
	}
	if cfg.DBPort, err = intValue("DB_PORT", 5432); err != nil {
		return Config{}, err
	}
	if cfg.DBUser, err = stringValue("DB_USER", "postgres"); err != nil {
		return Config{}, err
	}
	if cfg.DBPassword, err = stringValue("DB_PASSWORD", "postgres"); err != nil {
		return Config{}, err
	}
	if cfg.DBName, err = stringValue("DB_NAME", "orders_db"); err != nil {
		return Config{}, err
	}
	if cfg.DBMaxConns, err = int32Value("DB_MAX_CONNS", 20); err != nil {
		return Config{}, err
	}
	if cfg.DBMinConns, err = int32Value("DB_MIN_CONNS", 5); err != nil {
		return Config{}, err
	}
	if cfg.DBMaxConnLifetime, err = durationValue("DB_MAX_CONN_LIFETIME", 30*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.DBMaxConnIdleTime, err = durationValue("DB_MAX_CONN_IDLE_TIME", 5*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.DBHealthCheckPeriod, err = durationValue("DB_HEALTH_CHECK_PERIOD", time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.LogFormat, err = stringValue("LOG_FORMAT", "json"); err != nil {
		return Config{}, err
	}
	if cfg.LogLevel, err = stringValue("LOG_LEVEL", "info"); err != nil {
		return Config{}, err
	}

	if cfg.APIPort < 1 || cfg.APIPort > 65535 {
		return Config{}, fmt.Errorf("API_PORT debe estar entre 1 y 65535")
	}
	if cfg.DBPort < 1 || cfg.DBPort > 65535 {
		return Config{}, fmt.Errorf("DB_PORT debe estar entre 1 y 65535")
	}
	if cfg.DBMaxConns < 1 || cfg.DBMinConns < 0 || cfg.DBMinConns > cfg.DBMaxConns {
		return Config{}, fmt.Errorf("DB_MIN_CONNS debe ser mayor o igual a cero y no superar DB_MAX_CONNS")
	}
	if cfg.LogFormat != "json" && cfg.LogFormat != "text" {
		return Config{}, fmt.Errorf("LOG_FORMAT debe ser json o text")
	}
	if cfg.LogLevel != "debug" && cfg.LogLevel != "info" && cfg.LogLevel != "warn" && cfg.LogLevel != "error" {
		return Config{}, fmt.Errorf("LOG_LEVEL debe ser debug, info, warn o error")
	}
	if cfg.APIHost == "" || cfg.DBHost == "" || cfg.DBUser == "" || cfg.DBName == "" {
		return Config{}, fmt.Errorf("API_HOST, DB_HOST, DB_USER y DB_NAME no pueden estar vacios")
	}
	if cfg.RequestTimeout <= 0 ||
		cfg.ReadHeaderTimeout <= 0 ||
		cfg.ReadTimeout <= 0 ||
		cfg.WriteTimeout <= 0 ||
		cfg.IdleTimeout <= 0 ||
		cfg.ShutdownTimeout <= 0 ||
		cfg.DBMaxConnLifetime <= 0 ||
		cfg.DBMaxConnIdleTime <= 0 ||
		cfg.DBHealthCheckPeriod <= 0 {
		return Config{}, fmt.Errorf("los timeouts y periodos deben ser mayores que cero")
	}

	return cfg, nil
}

func stringValue(key, fallback string) (string, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback, nil
	}
	return value, nil
}

func intValue(key string, fallback int) (int, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s debe ser un entero: %w", key, err)
	}
	return parsed, nil
}

func int32Value(key string, fallback int32) (int32, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s debe ser un entero de 32 bits: %w", key, err)
	}
	return int32(parsed), nil
}

func durationValue(key string, fallback time.Duration) (time.Duration, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s debe ser una duracion valida: %w", key, err)
	}
	return parsed, nil
}
