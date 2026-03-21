package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	JWT      JWTConfig
	SMTP     SMTPConfig
	OAuth    OAuthConfig
	Security SecurityConfig
	Server   ServerConfig
}

type AppConfig struct {
	Name        string
	Env         string
	URL         string
	FrontendURL string
	Debug       bool
}

type DatabaseConfig struct {
	URL             string
	Host            string
	Port            int
	Name            string
	User            string
	Password        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type JWTConfig struct {
	Secret              string
	AccessTokenExpiry   time.Duration
	RefreshTokenExpiry  time.Duration
	Issuer              string
}

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	FromName string
	UseTLS   bool
}

type OAuthConfig struct {
	Google GoogleOAuthConfig
}

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

type SecurityConfig struct {
	BcryptCost         int
	RateLimitRequests  int
	RateLimitWindow    time.Duration
	CSRFSecret         string
	AllowedOrigins     []string
	CookieSecure       bool
	CookieSameSite     string
}

type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

var global *Config

// Load reads .env file and populates Config from environment variables.
func Load() (*Config, error) {
	// Load .env if present (ignore error — env vars may already be set)
	_ = godotenv.Load()

	cfg := &Config{}

	// ── App ───────────────────────────────────────────────────────────────────
	cfg.App = AppConfig{
		Name:        getEnv("APP_NAME", "ZATRANO"),
		Env:         getEnv("APP_ENV", "development"),
		URL:         getEnv("APP_URL", "http://localhost:8080"),
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
		Debug:       getEnvBool("APP_DEBUG", true),
	}

	// ── Database ──────────────────────────────────────────────────────────────
	dbURL := getEnv("DATABASE_URL", "")
	if dbURL == "" {
		dbURL = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
			getEnv("DB_HOST", "localhost"),
			getEnv("DB_PORT", "5432"),
			getEnv("DB_USER", "postgres"),
			getEnv("DB_PASSWORD", "postgres"),
			getEnv("DB_NAME", "zatrano"),
			getEnv("DB_SSLMODE", "disable"),
		)
	}
	cfg.Database = DatabaseConfig{
		URL:             dbURL,
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            getEnvInt("DB_PORT", 5432),
		Name:            getEnv("DB_NAME", "zatrano"),
		User:            getEnv("DB_USER", "postgres"),
		Password:        getEnv("DB_PASSWORD", "postgres"),
		SSLMode:         getEnv("DB_SSLMODE", "disable"),
		MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
		ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
	}

	// ── JWT ───────────────────────────────────────────────────────────────────
	jwtSecret := getEnv("JWT_SECRET", "")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is required")
	}
	cfg.JWT = JWTConfig{
		Secret:             jwtSecret,
		AccessTokenExpiry:  getEnvDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
		RefreshTokenExpiry: getEnvDuration("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		Issuer:             getEnv("JWT_ISSUER", "zatrano"),
	}

	// ── SMTP ──────────────────────────────────────────────────────────────────
	cfg.SMTP = SMTPConfig{
		Host:     getEnv("SMTP_HOST", "localhost"),
		Port:     getEnvInt("SMTP_PORT", 587),
		User:     getEnv("SMTP_USER", ""),
		Password: getEnv("SMTP_PASSWORD", ""),
		From:     getEnv("SMTP_FROM", "noreply@zatrano.com"),
		FromName: getEnv("SMTP_FROM_NAME", "ZATRANO"),
		UseTLS:   getEnvBool("SMTP_TLS", true),
	}

	// ── OAuth ─────────────────────────────────────────────────────────────────
	cfg.OAuth = OAuthConfig{
		Google: GoogleOAuthConfig{
			ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
			ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("GOOGLE_REDIRECT_URL", cfg.App.URL+"/api/v1/auth/google/callback"),
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
		},
	}

	// ── Security ──────────────────────────────────────────────────────────────
	csrfSecret := getEnv("CSRF_SECRET", "")
	if csrfSecret == "" {
		csrfSecret = jwtSecret + "_csrf"
	}
	cfg.Security = SecurityConfig{
		BcryptCost:        getEnvInt("BCRYPT_COST", 12),
		RateLimitRequests: getEnvInt("RATE_LIMIT_REQUESTS", 100),
		RateLimitWindow:   getEnvDuration("RATE_LIMIT_WINDOW", time.Minute),
		CSRFSecret:        csrfSecret,
		AllowedOrigins:    getEnvSlice("ALLOWED_ORIGINS", []string{cfg.App.FrontendURL}),
		CookieSecure:      getEnvBool("COOKIE_SECURE", cfg.App.Env == "production"),
		CookieSameSite:    getEnv("COOKIE_SAME_SITE", "Lax"),
	}

	// ── Server ────────────────────────────────────────────────────────────────
	cfg.Server = ServerConfig{
		Host:         getEnv("SERVER_HOST", "0.0.0.0"),
		Port:         getEnvInt("SERVER_PORT", 8080),
		ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
		WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:  getEnvDuration("SERVER_IDLE_TIMEOUT", 120*time.Second),
	}

	global = cfg
	return cfg, nil
}

// Get returns the global config instance. Panics if Load() was not called.
func Get() *Config {
	if global == nil {
		panic("config not loaded: call config.Load() first")
	}
	return global
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool { return c.App.Env == "development" }

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool { return c.App.Env == "production" }

// ServerAddress returns host:port string for the server.
func (c *Config) ServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// DSN returns a clean PostgreSQL DSN for GORM.
func (c *Config) DSN() string {
	return c.Database.URL
}

// ── helpers ────────────────────────────────────────────────────────────────────

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func getEnvSlice(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var result []string
	for _, s := range splitAndTrim(v, ",") {
		if s != "" {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}

func splitAndTrim(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, trimSpace(s[start:i]))
			start = i + len(sep)
		}
	}
	result = append(result, trimSpace(s[start:]))
	return result
}

func trimSpace(s string) string {
	i, j := 0, len(s)-1
	for i <= j && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	for j >= i && (s[j] == ' ' || s[j] == '\t') {
		j--
	}
	return s[i : j+1]
}
