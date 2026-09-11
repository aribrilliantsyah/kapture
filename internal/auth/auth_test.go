package auth

import (
	"encoding/json"
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
	if qr, err := GenerateQRCodePNG(url); err != nil || len(qr) == 0 {
		t.Fatalf("failed to generate QR code PNG: %v", err)
	}
	code, err := ComputeHOTP(secret, uint64(time.Now().Unix()/30))
	if err != nil || len(code) != 6 {
		t.Fatalf("compute hotp: %q %v", code, err)
	}
	if !VerifyTOTP(secret, code) {
		t.Errorf("expected code %s to be valid", code)
	}
	if VerifyTOTP(secret, "000000") && code != "000000" {
		t.Error("expected 000000 to be invalid")
	}
}

// codeAt returns the code of secret for the time step now+offset.
func codeAt(secret string, offset int64) string {
	c, _ := ComputeHOTP(secret, uint64(time.Now().Unix()/30+offset))
	return c
}

// setupAdmin runs the first-run setup and returns the manager and TOTP secret.
func setupAdmin(t *testing.T, file string) (*Manager, string) {
	t.Helper()
	m, err := NewManager(file, true)
	if err != nil {
		t.Fatal(err)
	}
	secret := GenerateSecret()
	if _, err := m.CompleteSetup("root", "Secret#123", secret, "999999", "first_pet", "Rex"); err == nil {
		t.Fatal("expected error on wrong OTP code")
	}
	if _, err := m.CompleteSetup("root", "Secret#123", secret, codeAt(secret, 0), "nope", "Rex"); err == nil {
		t.Fatal("expected error on unknown recovery question")
	}
	token, err := m.CompleteSetup("root", "Secret#123", secret, codeAt(secret, 0), "first_pet", "  Rex ")
	if err != nil || token == "" {
		t.Fatalf("complete setup: %v", err)
	}
	if id, ok := m.ValidateToken(token); !ok || id.Username != "root" || !id.IsAdmin() {
		t.Fatalf("setup session invalid: %+v %v", id, ok)
	}
	return m, secret
}

func TestSetupLoginAndRestart(t *testing.T) {
	file := filepath.Join(t.TempDir(), "auth.json")
	m, secret := setupAdmin(t, file)
	if !m.IsSetupCompleted() {
		t.Fatal("expected setup to be completed")
	}
	if _, err := m.Login("root", "wrongpass", codeAt(secret, 1)); err == nil {
		t.Fatal("expected error on wrong password")
	}
	// The setup code was consumed: replaying it must fail.
	if _, err := m.Login("root", "Secret#123", codeAt(secret, 0)); err == nil {
		t.Fatal("expected a replayed TOTP code to be rejected")
	}
	// Usernames are case-insensitive.
	token, err := m.Login("ROOT", "Secret#123", codeAt(secret, 1))
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	m2, err := NewManager(file, true)
	if err != nil {
		t.Fatal(err)
	}
	if id, ok := m2.ValidateToken(token); !ok || id.Username != "root" {
		t.Fatal("session must survive a restart")
	}
	if _, ok := m2.ValidateToken(token + "x"); ok {
		t.Fatal("tampered token accepted")
	}
	m2.Logout(token)
	if _, ok := m2.ValidateToken(token); ok {
		t.Fatal("revoked token accepted")
	}
}

func TestExpiredStepAsksForPasswordAgain(t *testing.T) {
	m, secret := setupAdmin(t, filepath.Join(t.TempDir(), "auth.json"))
	res, err := m.BeginLogin("root", "Secret#123")
	if err != nil || res.Step != StepTOTP || res.ExpiresIn <= 0 {
		t.Fatalf("begin login: %+v %v", res, err)
	}
	m.pending[res.TempToken].expires = time.Now().Add(-time.Second)
	if _, err := m.ContinueTOTP(res.TempToken, codeAt(secret, 1)); err != ErrStepExpired {
		t.Fatalf("expected ErrStepExpired, got %v", err)
	}
}

func TestNewOperatorFirstSignIn(t *testing.T) {
	m, _ := setupAdmin(t, filepath.Join(t.TempDir(), "auth.json"))
	op, err := m.CreateUser("ops", "Ops Team", RoleOperator, "Temp#pass1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.CreateUser("OPS", "", RoleOperator, "Temp#pass1"); err == nil {
		t.Fatal("duplicate username accepted")
	}
	if _, err := m.Login("ops", "Temp#pass1", ""); err == nil {
		t.Fatal("script login must refuse an account that has not finished sign-in")
	}

	// Temporary password: choose a new one, then enroll an authenticator.
	res, err := m.BeginLogin("ops", "Temp#pass1")
	if err != nil || res.Step != StepPassword {
		t.Fatalf("expected password step: %+v %v", res, err)
	}
	if _, err := m.ContinueTOTP(res.TempToken, "123456"); err == nil {
		t.Fatal("out-of-order step accepted")
	}
	if _, err := m.ContinuePassword(res.TempToken, "Temp#pass1"); err == nil {
		t.Fatal("reusing the temporary password must be refused")
	}
	res, err = m.ContinuePassword(res.TempToken, "Operator#1")
	if err != nil || res.Step != StepEnroll || res.Secret == "" {
		t.Fatalf("expected enroll step: %+v %v", res, err)
	}
	res, err = m.ContinueEnroll(res.TempToken, codeAt(res.Secret, 0))
	if err != nil || res.Step != StepDone || res.Session == "" {
		t.Fatalf("expected session: %+v %v", res, err)
	}
	id, ok := m.ValidateToken(res.Session)
	if !ok || id.Role != RoleOperator || id.IsAdmin() || id.DisplayName != "Ops Team" {
		t.Fatalf("operator identity: %+v", id)
	}

	// An admin reset of the 2FA ends the operator's sessions.
	if err := m.Reset2FA(op.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.ValidateToken(res.Session); ok {
		t.Fatal("session must end after a 2FA reset")
	}
	if res, _ := m.BeginLogin("ops", "Operator#1"); res.Step != StepEnroll {
		t.Fatalf("expected enrollment after reset, got %s", res.Step)
	}
	// Operators have no recovery question.
	if _, _, err := m.BeginRecovery("ops"); err != ErrNoRecovery {
		t.Fatalf("operators must not recover themselves: %v", err)
	}
}

func TestRecoveryNeedsAnswerAndSecondFactor(t *testing.T) {
	m, secret := setupAdmin(t, filepath.Join(t.TempDir(), "auth.json"))

	q, res, err := m.BeginRecovery("root")
	if err != nil || q == "" {
		t.Fatalf("begin recovery: %v", err)
	}
	// The answer alone is not enough.
	if _, err := m.FinishRecovery(res.TempToken, "rex", "password", "000000", ""); err == nil {
		t.Fatal("password reset without a valid 2FA code")
	}
	if _, err := m.FinishRecovery(res.TempToken, "wrong", "2fa", "", "Secret#123"); err == nil {
		t.Fatal("wrong answer accepted")
	}
	// Answer (case and spacing ignored) + password: reset 2FA.
	next, err := m.FinishRecovery(res.TempToken, "REX", "2fa", "", "Secret#123")
	if err != nil || next.Step != StepEnroll {
		t.Fatalf("expected enroll step: %+v %v", next, err)
	}
	done, err := m.ContinueEnroll(next.TempToken, codeAt(next.Secret, 0))
	if err != nil || done.Step != StepDone {
		t.Fatalf("enroll: %+v %v", done, err)
	}
	if _, ok := m.ValidateToken(done.Session); !ok {
		t.Fatal("recovered session invalid")
	}
	if !VerifyTOTP(next.Secret, codeAt(next.Secret, 0)) || next.Secret == secret {
		t.Fatal("expected a new authenticator key")
	}

	// Answer + current code: reset the password.
	_, res, _ = m.BeginRecovery("root")
	next, err = m.FinishRecovery(res.TempToken, "rex", "password", codeAt(next.Secret, 1), "")
	if err != nil || next.Step != StepPassword {
		t.Fatalf("expected password step: %+v %v", next, err)
	}
	if done, err = m.ContinuePassword(next.TempToken, "BrandNew#9"); err != nil || done.Step != StepDone {
		t.Fatalf("set password: %+v %v", done, err)
	}

	// Too many wrong answers end the attempt.
	_, res, _ = m.BeginRecovery("root")
	for i := 0; i < maxStepTries; i++ {
		m.FinishRecovery(res.TempToken, "nope", "2fa", "", "BrandNew#9")
	}
	if _, err := m.FinishRecovery(res.TempToken, "rex", "2fa", "", "BrandNew#9"); err != ErrStepExpired {
		t.Fatalf("expected the attempt to end, got %v", err)
	}
}

func TestLastAdminIsProtected(t *testing.T) {
	m, _ := setupAdmin(t, filepath.Join(t.TempDir(), "auth.json"))
	root := m.ListUsers()[0]
	op := RoleOperator
	if _, err := m.UpdateUser(root.ID, UserPatch{Role: &op}); err == nil {
		t.Fatal("demoted the last administrator")
	}
	if err := m.DeleteUser(root.ID); err == nil {
		t.Fatal("deleted the last administrator")
	}
	second, _ := m.CreateUser("second", "", RoleAdmin, "Password#22")
	if _, err := m.UpdateUser(root.ID, UserPatch{Role: &op}); err != nil {
		t.Fatalf("demote with another admin present: %v", err)
	}
	if err := m.DeleteUser(second.ID); err == nil {
		t.Fatal("deleted the only remaining administrator")
	}
}

func TestProfileChanges(t *testing.T) {
	m, secret := setupAdmin(t, filepath.Join(t.TempDir(), "auth.json"))
	old, _ := m.Login("root", "Secret#123", codeAt(secret, 1))
	id, _ := m.ValidateToken(old)

	if _, err := m.ChangePassword(id.ID, "bad", "Another#22"); err == nil {
		t.Fatal("wrong current password accepted")
	}
	fresh, err := m.ChangePassword(id.ID, "Secret#123", "Another#22")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.ValidateToken(old); ok {
		t.Fatal("old sessions must end after a password change")
	}
	if _, ok := m.ValidateToken(fresh); !ok {
		t.Fatal("the new session must stay valid")
	}
	if s, _, err := m.RevealTOTP(id.ID, "Another#22"); err != nil || s != secret {
		t.Fatalf("reveal: %v", err)
	}
	if _, _, err := m.RevealTOTP(id.ID, "Secret#123"); err == nil {
		t.Fatal("revealed the key with a wrong password")
	}
	if err := m.SetRecovery(id.ID, "Another#22", "birth_city", "Yogyakarta"); err != nil {
		t.Fatal(err)
	}
	if info, _ := m.GetUser(id.ID); info.RecoveryQuestion != "birth_city" {
		t.Fatalf("recovery not saved: %+v", info)
	}
}

func TestMigratesSingleAdminFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "auth.json")
	old, _ := json.Marshal(map[string]any{
		"setup_completed": true, "username": "admin", "password_hash": mustHash(t, "Secret#123"),
		"totp_secret": GenerateSecret(), "session_key": randomHex(32),
	})
	if err := os.WriteFile(file, old, 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(file, true)
	if err != nil {
		t.Fatal(err)
	}
	users := m.ListUsers()
	if len(users) != 1 || users[0].Username != "admin" || users[0].Role != RoleAdmin || !users[0].Has2FA {
		t.Fatalf("migration: %+v", users)
	}
	if !m.IsSetupCompleted() {
		t.Fatal("migrated admin must count as set up")
	}
}

func TestLoginLockout(t *testing.T) {
	mgr, _ := NewManager(filepath.Join(t.TempDir(), "auth.json"), true)
	for i := 0; i < maxFailures; i++ {
		if err := mgr.Allow("10.0.0.1"); err != nil {
			t.Fatalf("locked too early at attempt %d", i)
		}
		mgr.Fail("10.0.0.1")
	}
	if mgr.Allow("10.0.0.1") == nil {
		t.Fatal("expected lockout after repeated failures")
	}
	if mgr.Allow("10.0.0.2") != nil {
		t.Fatal("other clients must not be locked")
	}
}

func TestPasswordPolicy(t *testing.T) {
	for pw, ok := range map[string]bool{
		"Qawsed#1477": true, "Pässwörd#9": true, "Aa1!aaaa": true,
		"Aa1!aaa": false, "qawsed#1477": false, "QAWSED#1477": false, "Qawsed#abcd": false, "Qawsed14777": false,
		"Qawsed 1477": false, // a space is not a symbol
	} {
		if err := checkPassword(pw); (err == nil) != ok {
			t.Errorf("checkPassword(%q) = %v, want ok=%v", pw, err, ok)
		}
	}
	m, _ := setupAdmin(t, filepath.Join(t.TempDir(), "auth.json"))
	if _, err := m.CreateUser("weak", "", RoleOperator, "password1"); err == nil {
		t.Fatal("created a user with a weak password")
	}
}

func TestAdminSetsRecoveryOfAnotherAdmin(t *testing.T) {
	m, _ := setupAdmin(t, filepath.Join(t.TempDir(), "auth.json"))
	second, _ := m.CreateUser("second", "", RoleAdmin, "Password#22")
	op, _ := m.CreateUser("ops", "", RoleOperator, "Password#22")
	if err := m.SetRecoveryFor(op.ID, "birth_city", "Yogya"); err == nil {
		t.Fatal("operators must not get a recovery question")
	}
	if err := m.SetRecoveryFor(second.ID, "birth_city", "Yogya"); err != nil {
		t.Fatal(err)
	}
	if q, _, err := m.BeginRecovery("second"); err != nil || q == "" {
		t.Fatalf("recovery after an admin set the question: %q %v", q, err)
	}
}

func mustHash(t *testing.T, s string) string {
	t.Helper()
	h, err := hashPassword(s)
	if err != nil {
		t.Fatal(err)
	}
	return h
}
