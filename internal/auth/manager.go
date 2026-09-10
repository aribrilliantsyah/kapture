package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// StoredAuth defines the persistent credentials structure saved to disk.
type StoredAuth struct {
	SetupCompleted bool      `json:"setup_completed"`
	Username       string    `json:"username"`
	PasswordHash   string    `json:"password_hash"`
	TOTPSecret     string    `json:"totp_secret"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Session represents an active user login session.
type Session struct {
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
}

// TempLogin holds a temporary verification challenge for 2-step login.
type TempLogin struct {
	Username  string
	ExpiresAt time.Time
}

// Manager handles authentication, setup onboarding, and session verification.
type Manager struct {
	filePath   string
	enabled    bool
	auth       StoredAuth
	sessions   map[string]Session
	tempLogins map[string]TempLogin
	mu         sync.RWMutex
}

// NewManager creates a new Auth Manager.
func NewManager(filePath string, enabled bool) (*Manager, error) {
	if filePath == "" {
		filePath = "/data/logcatcher/auth.json"
	}

	// Test if directory is writable, fallback to local path if not
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("configured auth path not writable, falling back to local data directory", "error", err, "path", filePath)
		filePath = "data/auth.json"
		_ = os.MkdirAll("data", 0o755)
	}

	m := &Manager{
		filePath:   filePath,
		enabled:    enabled,
		sessions:   make(map[string]Session),
		tempLogins: make(map[string]TempLogin),
	}

	if !enabled {
		return m, nil
	}

	// Try loading existing credentials from disk
	if err := m.load(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Info("no existing auth config found, setup wizard will be required", "file", filePath)
		} else {
			slog.Warn("failed to load auth file, starting in setup mode", "error", err, "file", filePath)
		}
	}

	// Periodically clean expired sessions
	go m.cleanupSessionsLoop()

	return m, nil
}

// IsEnabled returns true if authentication is active.
func (m *Manager) IsEnabled() bool {
	return m.enabled
}

// IsSetupCompleted returns true if initial setup has been completed.
func (m *Manager) IsSetupCompleted() bool {
	if !m.enabled {
		return true
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.auth.SetupCompleted && m.auth.Username != "" && m.auth.PasswordHash != "" && m.auth.TOTPSecret != ""
}

// GetUsername returns the configured administrator username.
func (m *Manager) GetUsername() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.auth.Username
}

// CompleteSetup saves new credentials and verifies the first TOTP code.
func (m *Manager) CompleteSetup(username, password, secret, otpCode string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	secret = strings.TrimSpace(secret)
	otpCode = strings.TrimSpace(otpCode)

	if username == "" {
		return "", errors.New("username cannot be empty")
	}
	if len(password) < 6 {
		return "", errors.New("password must be at least 6 characters")
	}
	if secret == "" {
		return "", errors.New("2FA secret is required")
	}

	// Verify the 2FA code with the secret
	if !VerifyTOTP(secret, otpCode) {
		return "", errors.New("invalid 2FA code. Please check your Google Authenticator app")
	}

	// Hash password using bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	m.auth = StoredAuth{
		SetupCompleted: true,
		Username:       username,
		PasswordHash:   string(hash),
		TOTPSecret:     secret,
		UpdatedAt:      time.Now().UTC(),
	}

	// Save to disk
	if err := m.save(); err != nil {
		return "", fmt.Errorf("save auth file: %w", err)
	}

	slog.Info("initial setup completed successfully with 2FA enabled", "username", username)

	// Create initial session token
	token := m.createSessionLocked(username)
	return token, nil
}

// ValidateCredentials verifies username and password, returning a 5-minute temporary token for Step 2 (2FA).
func (m *Manager) ValidateCredentials(username, password string) (string, error) {
	if !m.enabled {
		return "anonymous-temp", nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.auth.SetupCompleted {
		return "", errors.New("setup belum diselesaikan")
	}

	if strings.TrimSpace(username) != m.auth.Username {
		return "", errors.New("username atau password salah")
	}

	// Check password bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(m.auth.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("username atau password salah")
	}

	// Generate 5-minute temporary challenge token for 2FA code verification
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	tempToken := hex.EncodeToString(b)

	m.tempLogins[tempToken] = TempLogin{
		Username:  m.auth.Username,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	return tempToken, nil
}

// Verify2FALogin validates the 6-digit TOTP code using the tempToken from ValidateCredentials.
func (m *Manager) Verify2FALogin(tempToken, otpCode string) (string, error) {
	if !m.enabled {
		return "anonymous-token", nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	tl, exists := m.tempLogins[tempToken]
	if !exists || time.Now().After(tl.ExpiresAt) {
		delete(m.tempLogins, tempToken)
		return "", errors.New("sesi verifikasi 2FA kedaluwarsa. Silakan masukkan username & password kembali")
	}

	if !VerifyTOTP(m.auth.TOTPSecret, otpCode) {
		return "", errors.New("kode 2FA salah atau kedaluwarsa. Periksa Google Authenticator Anda")
	}

	// Remove temporary token
	delete(m.tempLogins, tempToken)

	// Create full 24-hour session
	token := m.createSessionLocked(tl.Username)
	slog.Info("user logged in successfully with 2FA", "username", tl.Username)
	return token, nil
}

// Login validates username, password, and 2FA OTP code in a single call (for API/scripts).
func (m *Manager) Login(username, password, otpCode string) (string, error) {
	tempToken, err := m.ValidateCredentials(username, password)
	if err != nil {
		return "", err
	}
	return m.Verify2FALogin(tempToken, otpCode)
}

// ValidateToken verifies a session token.
func (m *Manager) ValidateToken(token string) (bool, string) {
	if !m.enabled {
		return true, "anonymous"
	}
	if token == "" {
		return false, ""
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[token]
	if !exists {
		return false, ""
	}
	if time.Now().After(session.ExpiresAt) {
		return false, ""
	}

	return true, session.Username
}

// Logout invalidates a session token.
func (m *Manager) Logout(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
}

func (m *Manager) createSessionLocked(username string) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)

	m.sessions[token] = Session{
		Username:  username,
		ExpiresAt: time.Now().Add(24 * time.Hour), // 24 hours valid
	}
	return token
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &m.auth)
}

func (m *Manager) save() error {
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m.auth, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0o600)
}

func (m *Manager) cleanupSessionsLoop() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for token, sess := range m.sessions {
			if now.After(sess.ExpiresAt) {
				delete(m.sessions, token)
			}
		}
		for tempTok, tl := range m.tempLogins {
			if now.After(tl.ExpiresAt) {
				delete(m.tempLogins, tempTok)
			}
		}
		m.mu.Unlock()
	}
}
