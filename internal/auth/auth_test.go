package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTOTPGenerationAndVerification(t *testing.T) {
	secret := GenerateSecret()
	if len(secret) != 32 {
		t.Fatalf("expected 32-character base32 secret, got %d (%s)", len(secret), secret)
	}

	url := GenerateOTPAuthURL("admin", secret)
	if url == "" {
		t.Fatal("expected non-empty otpauth url")
	}

	qr, err := GenerateQRCodePNG(url)
	if err != nil || len(qr) == 0 {
		t.Fatalf("failed to generate QR code PNG: %v", err)
	}

	// Compute current valid code
	counter := uint64(time.Now().Unix() / 30)
	code, err := ComputeHOTP(secret, counter)
	if err != nil {
		t.Fatalf("compute hotp: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("expected 6-digit code, got %s", code)
	}

	// Verify valid code
	if !VerifyTOTP(secret, code) {
		t.Errorf("expected code %s to be valid for secret %s", code, secret)
	}

	// Verify invalid code
	if VerifyTOTP(secret, "000000") && code != "000000" {
		t.Error("expected 000000 to be invalid")
	}
}

func TestAuthManagerSetupAndLogin(t *testing.T) {
	dir := t.TempDir()
	authFile := filepath.Join(dir, "auth.json")

	mgr, err := NewManager(authFile, true)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Initially not setup
	if mgr.IsSetupCompleted() {
		t.Fatal("expected setup not completed initially")
	}

	secret := GenerateSecret()
	now := uint64(time.Now().Unix() / 30)
	code, _ := ComputeHOTP(secret, now)

	// Complete setup with wrong code
	_, err = mgr.CompleteSetup("superadmin", "secret123", secret, "999999")
	if err == nil {
		t.Fatal("expected error on wrong OTP code")
	}

	// Complete setup with correct code
	token, err := mgr.CompleteSetup("superadmin", "secret123", secret, code)
	if err != nil {
		t.Fatalf("complete setup failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Verify setup is completed
	if !mgr.IsSetupCompleted() {
		t.Fatal("expected setup to be completed")
	}

	// Verify token
	valid, username := mgr.ValidateToken(token)
	if !valid || username != "superadmin" {
		t.Fatalf("expected valid token for superadmin, got valid=%v user=%s", valid, username)
	}

	// Test login with wrong password
	_, err = mgr.Login("superadmin", "wrongpass", code)
	if err == nil {
		t.Fatal("expected error on wrong password")
	}

	// Test login with correct password and code
	loginToken, err := mgr.Login("superadmin", "secret123", code)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if loginToken == "" {
		t.Fatal("expected non-empty login token")
	}

	// Test reload from disk
	mgr2, err := NewManager(authFile, true)
	if err != nil {
		t.Fatalf("reload manager: %v", err)
	}
	if !mgr2.IsSetupCompleted() {
		t.Fatal("expected reloaded manager to recognize completed setup")
	}
	if mgr2.GetUsername() != "superadmin" {
		t.Fatalf("expected superadmin, got %s", mgr2.GetUsername())
	}
}

// Silence unused os import
var _ = os.TempDir
