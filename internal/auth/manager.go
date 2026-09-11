package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	// SessionTTL is how long a session lasts without activity.
	SessionTTL = 30 * 24 * time.Hour
	// Sessions older than this are re-issued on use (sliding expiry).
	refreshAfter = 24 * time.Hour
	// StepTTL is how long a half-finished sign-in (or recovery) stays valid.
	StepTTL = 5 * time.Minute

	maxFailures   = 5
	failureWindow = 10 * time.Minute
	lockDuration  = 5 * time.Minute
	maxStepTries  = 5

	// MinPasswordLen is the shortest password accepted.
	MinPasswordLen = 8
	fileVersion    = 2
)

// Roles. Operators can use everything except user management.
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
)

// Sign-in steps that follow a correct password or recovery answer.
const (
	StepTOTP     = "totp"     // enter the authenticator code
	StepPassword = "password" // choose a new password
	StepEnroll   = "enroll"   // scan a new authenticator key
	StepDone     = "done"
	stepRecover  = "recover" // answer the recovery question
)

// Errors the handlers map to specific replies.
var (
	ErrStepExpired  = errors.New("your sign-in took too long, enter your password again")
	ErrInvalidLogin = errors.New("invalid username or password")
	ErrNotFound     = errors.New("user not found")
	ErrNoRecovery   = errors.New("password recovery is not available for this account, ask an administrator to reset it")
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._@-]{1,31}$`)

// User is one account as stored in auth.json.
type User struct {
	ID                 string     `json:"id"`
	Username           string     `json:"username"`
	DisplayName        string     `json:"display_name,omitempty"`
	Role               string     `json:"role"`
	PasswordHash       string     `json:"password_hash"`
	TOTPSecret         string     `json:"totp_secret,omitempty"` // empty: enroll at next sign-in
	MustChangePassword bool       `json:"must_change_password,omitempty"`
	RecoveryQuestion   string     `json:"recovery_question,omitempty"`
	RecoveryAnswerHash string     `json:"recovery_answer_hash,omitempty"`
	SessionGen         int        `json:"session_gen"` // bumped to end every session of the user
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	LastLoginAt        *time.Time `json:"last_login_at,omitempty"`
}

// UserInfo is the public view of a user.
type UserInfo struct {
	ID                 string     `json:"id"`
	Username           string     `json:"username"`
	DisplayName        string     `json:"display_name"`
	Role               string     `json:"role"`
	Has2FA             bool       `json:"has_2fa"`
	MustChangePassword bool       `json:"must_change_password"`
	HasRecovery        bool       `json:"has_recovery"`
	RecoveryQuestion   string     `json:"recovery_question,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	LastLoginAt        *time.Time `json:"last_login_at,omitempty"`
}

// Identity is the signed-in user of a request.
type Identity struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

// IsAdmin reports whether the identity may manage users.
func (i Identity) IsAdmin() bool { return i.Role == RoleAdmin }

// Anonymous is the identity used when authentication is disabled.
var Anonymous = Identity{ID: "anonymous", Username: "anonymous", DisplayName: "Anonymous", Role: RoleAdmin}

// StoredAuth is the content of auth.json.
type StoredAuth struct {
	Version int     `json:"version"`
	Users   []*User `json:"users"`
	// SessionKey signs session tokens. Keeping it on disk is what lets
	// sessions survive an aggregator restart.
	SessionKey string    `json:"session_key"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Version 1 (single administrator) fields, migrated on load.
	SetupCompleted bool   `json:"setup_completed,omitempty"`
	Username       string `json:"username,omitempty"`
	PasswordHash   string `json:"password_hash,omitempty"`
	TOTPSecret     string `json:"totp_secret,omitempty"`
}

// StepResult tells the sign-in page what to show next.
type StepResult struct {
	Step       string `json:"step"`
	TempToken  string `json:"temp_token,omitempty"`
	Username   string `json:"username,omitempty"`
	ExpiresIn  int    `json:"expires_in,omitempty"` // seconds left for this sign-in
	Secret     string `json:"secret,omitempty"`     // StepEnroll only
	OTPAuthURL string `json:"otpauth_url,omitempty"`
	Session    string `json:"-"` // set when Step is StepDone
}

// pending is a sign-in (or recovery) between its steps.
type pending struct {
	userID  string
	steps   []string
	secret  string // new authenticator key during StepEnroll
	expires time.Time
	tries   int
}

type session struct {
	User    string `json:"u"` // user id
	Gen     int    `json:"g"`
	ID      string `json:"i"`
	Issued  int64  `json:"t"`
	Expires int64  `json:"e"`
}

type failure struct {
	count       int
	first       time.Time
	lockedUntil time.Time
}

// Manager handles accounts, sign-in with TOTP, recovery and signed session tokens.
type Manager struct {
	filePath string
	enabled  bool
	data     StoredAuth
	key      []byte
	pending  map[string]*pending
	revoked  map[string]int64 // session id -> expiry
	failures map[string]*failure
	lastOTP  map[string]int64 // user id -> last accepted TOTP counter, blocks code replay
	mu       sync.Mutex
}

// NewManager loads auth.json (or starts in setup mode when there is none).
func NewManager(filePath string, enabled bool) (*Manager, error) {
	if filePath == "" {
		filePath = "/data/kapture/auth.json"
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		slog.Warn("auth path not writable, falling back to ./data", "error", err, "path", filePath)
		filePath = "data/auth.json"
		_ = os.MkdirAll("data", 0o755)
	}

	m := &Manager{
		filePath: filePath,
		enabled:  enabled,
		pending:  map[string]*pending{},
		revoked:  map[string]int64{},
		failures: map[string]*failure{},
		lastOTP:  map[string]int64{},
	}
	if !enabled {
		return m, nil
	}

	if err := m.load(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Info("no auth config found, setup is required", "file", filePath)
		} else {
			slog.Warn("failed to load auth file, starting in setup mode", "error", err, "file", filePath)
		}
	}
	dirty := m.migrate()
	if len(m.data.Users) > 0 && m.data.SessionKey == "" {
		m.data.SessionKey = randomHex(32)
		dirty = true
	}
	if dirty {
		if err := m.save(); err != nil {
			slog.Warn("cannot persist auth file, changes will not survive a restart", "error", err)
		}
	}
	m.key, _ = hex.DecodeString(m.data.SessionKey)

	go m.cleanupLoop()
	return m, nil
}

// migrate turns a version 1 file (one administrator) into the user list.
func (m *Manager) migrate() bool {
	d := &m.data
	if d.Version >= fileVersion {
		return false
	}
	if d.SetupCompleted && d.Username != "" && d.PasswordHash != "" {
		now := time.Now().UTC()
		d.Users = []*User{{
			ID: randomHex(8), Username: d.Username, Role: RoleAdmin,
			PasswordHash: d.PasswordHash, TOTPSecret: d.TOTPSecret, CreatedAt: now, UpdatedAt: now,
		}}
		slog.Info("auth file upgraded to multi-user, existing sessions must sign in again", "admin", d.Username)
	}
	d.Version = fileVersion
	d.SetupCompleted, d.Username, d.PasswordHash, d.TOTPSecret = false, "", "", ""
	return len(d.Users) > 0
}

// IsEnabled returns true if authentication is active.
func (m *Manager) IsEnabled() bool { return m.enabled }

// IsSetupCompleted reports whether the first administrator exists.
func (m *Manager) IsSetupCompleted() bool {
	if !m.enabled {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.setupDoneLocked()
}

func (m *Manager) setupDoneLocked() bool {
	for _, u := range m.data.Users {
		if u.Role == RoleAdmin && u.PasswordHash != "" && u.TOTPSecret != "" {
			return true
		}
	}
	return false
}

// ── First run ──

// CompleteSetup creates the first administrator after verifying its first TOTP
// code and returns a session token.
func (m *Manager) CompleteSetup(username, password, secret, otpCode, question, answer string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.setupDoneLocked() {
		return "", errors.New("setup has already been completed")
	}
	username = strings.TrimSpace(username)
	if err := checkUsername(username); err != nil {
		return "", err
	}
	if err := checkPassword(password); err != nil {
		return "", err
	}
	answerHash, err := recoveryHash(question, answer)
	if err != nil {
		return "", err
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", errors.New("2FA secret is required")
	}
	counter, ok := matchTOTP(secret, otpCode)
	if !ok {
		return "", errors.New("invalid 2FA code, check the time on your phone and try again")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	u := &User{
		ID: randomHex(8), Username: username, Role: RoleAdmin, PasswordHash: hash, TOTPSecret: secret,
		RecoveryQuestion: question, RecoveryAnswerHash: answerHash, CreatedAt: now, UpdatedAt: now, LastLoginAt: &now,
	}
	m.data.Users = []*User{u}
	m.data.SessionKey = randomHex(32)
	m.key, _ = hex.DecodeString(m.data.SessionKey)
	m.lastOTP[otpKey(u.ID, secret)] = counter
	if err := m.save(); err != nil {
		return "", fmt.Errorf("save auth file: %w", err)
	}
	slog.Info("initial setup completed with 2FA", "username", username)
	return m.newSessionLocked(u), nil
}

// ── Sign in ──

// BeginLogin checks username and password and returns the next step.
func (m *Manager) BeginLogin(username, password string) (*StepResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.setupDoneLocked() {
		return nil, errors.New("setup has not been completed")
	}
	u := m.byNameLocked(username)
	hash := dummyHash()
	if u != nil {
		hash = u.PasswordHash
	}
	// bcrypt always runs, so an unknown username takes as long as a wrong password.
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil || u == nil {
		return nil, ErrInvalidLogin
	}
	var steps []string
	if u.TOTPSecret != "" {
		steps = append(steps, StepTOTP)
	}
	if u.MustChangePassword {
		steps = append(steps, StepPassword)
	}
	if u.TOTPSecret == "" {
		steps = append(steps, StepEnroll)
	}
	return m.startLocked(u, steps)
}

// ContinueTOTP finishes the code step.
func (m *Manager) ContinueTOTP(temp, code string) (*StepResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, u, err := m.takeLocked(temp, StepTOTP)
	if err != nil {
		return nil, err
	}
	if err := m.checkCodeLocked(u, u.TOTPSecret, code); err != nil {
		m.tryLocked(temp, p)
		return nil, err
	}
	return m.advanceLocked(temp, p, u)
}

// ContinuePassword sets the new password the account was asked to choose.
func (m *Manager) ContinuePassword(temp, password string) (*StepResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, u, err := m.takeLocked(temp, StepPassword)
	if err != nil {
		return nil, err
	}
	if err := checkPassword(password); err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil {
		return nil, errors.New("choose a password different from the current one")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	u.PasswordHash, u.MustChangePassword = hash, false
	u.SessionGen++
	m.touchLocked(u)
	return m.advanceLocked(temp, p, u)
}

// ContinueEnroll saves the new authenticator key once a code from it matches.
func (m *Manager) ContinueEnroll(temp, code string) (*StepResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, u, err := m.takeLocked(temp, StepEnroll)
	if err != nil {
		return nil, err
	}
	if err := m.checkCodeLocked(u, p.secret, code); err != nil {
		m.tryLocked(temp, p)
		return nil, err
	}
	u.TOTPSecret = p.secret
	u.SessionGen++
	m.touchLocked(u)
	slog.Info("authenticator enrolled", "username", u.Username)
	return m.advanceLocked(temp, p, u)
}

// Login signs in with username, password and code in one call (API/scripts).
// Accounts with a pending password change or enrollment must use the dashboard.
func (m *Manager) Login(username, password, code string) (string, error) {
	res, err := m.BeginLogin(username, password)
	if err != nil {
		return "", err
	}
	if res.Step == StepTOTP {
		if res, err = m.ContinueTOTP(res.TempToken, code); err != nil {
			return "", err
		}
	}
	if res.Step != StepDone {
		return "", errors.New("this account must finish signing in on the dashboard first")
	}
	return res.Session, nil
}

// ── Recovery (administrators only) ──

// BeginRecovery returns the recovery question of an administrator.
func (m *Manager) BeginRecovery(username string) (question string, res *StepResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byNameLocked(username)
	if u == nil || u.Role != RoleAdmin || u.RecoveryAnswerHash == "" {
		return "", nil, ErrNoRecovery
	}
	text, _ := QuestionText(u.RecoveryQuestion)
	res, err = m.startLocked(u, []string{stepRecover})
	return text, res, err
}

// FinishRecovery checks the answer plus one other factor: the authenticator
// code to reset the password, or the password to reset two-factor sign-in.
// The answer alone never unlocks both, so it cannot take over the account.
func (m *Manager) FinishRecovery(temp, answer, mode, code, password string) (*StepResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, u, err := m.takeLocked(temp, stepRecover)
	if err != nil {
		return nil, err
	}
	answerOK := bcrypt.CompareHashAndPassword([]byte(u.RecoveryAnswerHash), []byte(normalizeAnswer(answer))) == nil
	var next string
	switch mode {
	case "password":
		if !answerOK || u.TOTPSecret == "" || m.checkCodeLocked(u, u.TOTPSecret, code) != nil {
			m.tryLocked(temp, p)
			return nil, errors.New("the answer or the 2FA code is wrong")
		}
		next = StepPassword
	case "2fa":
		passOK := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
		if !answerOK || !passOK {
			m.tryLocked(temp, p)
			return nil, errors.New("the answer or the password is wrong")
		}
		next = StepEnroll
	default:
		return nil, errors.New("choose what to reset: password or 2fa")
	}
	slog.Warn("account recovery answered", "username", u.Username, "reset", mode)
	p.steps = []string{stepRecover, next}
	return m.advanceLocked(temp, p, u)
}

// ── Step helpers ──

func (m *Manager) startLocked(u *User, steps []string) (*StepResult, error) {
	temp := randomHex(24)
	p := &pending{userID: u.ID, steps: append([]string{""}, steps...), expires: time.Now().Add(StepTTL)}
	return m.advanceLocked(temp, p, u)
}

// advanceLocked drops the finished step and describes the next one, or
// finishes the sign-in with a session.
func (m *Manager) advanceLocked(temp string, p *pending, u *User) (*StepResult, error) {
	p.steps = p.steps[1:]
	p.tries = 0
	res := &StepResult{Username: u.Username}
	if len(p.steps) == 0 {
		delete(m.pending, temp)
		now := time.Now().UTC()
		u.LastLoginAt = &now
		if err := m.save(); err != nil {
			return nil, fmt.Errorf("save auth file: %w", err)
		}
		slog.Info("user signed in", "username", u.Username)
		res.Step, res.Session = StepDone, m.newSessionLocked(u)
		return res, nil
	}
	m.pending[temp] = p
	res.Step, res.TempToken = p.steps[0], temp
	res.ExpiresIn = int(time.Until(p.expires).Seconds())
	if res.Step == StepEnroll {
		if p.secret == "" {
			p.secret = GenerateSecret()
		}
		res.Secret, res.OTPAuthURL = p.secret, GenerateOTPAuthURL(u.Username, p.secret)
	}
	return res, nil
}

func (m *Manager) takeLocked(temp, step string) (*pending, *User, error) {
	p, ok := m.pending[temp]
	if !ok || time.Now().After(p.expires) {
		delete(m.pending, temp)
		return nil, nil, ErrStepExpired
	}
	u := m.byIDLocked(p.userID)
	if u == nil {
		delete(m.pending, temp)
		return nil, nil, ErrStepExpired
	}
	if len(p.steps) == 0 || p.steps[0] != step {
		return nil, nil, errors.New("this sign-in step is not expected now, start again")
	}
	return p, u, nil
}

// tryLocked counts a wrong answer; too many end the sign-in.
func (m *Manager) tryLocked(temp string, p *pending) {
	if p.tries++; p.tries >= maxStepTries {
		delete(m.pending, temp)
	}
}

func (m *Manager) checkCodeLocked(u *User, secret, code string) error {
	counter, ok := matchTOTP(secret, code)
	if !ok {
		return errors.New("invalid 2FA code")
	}
	key := otpKey(u.ID, secret)
	if counter <= m.lastOTP[key] {
		return errors.New("this code was already used, wait for the next one")
	}
	m.lastOTP[key] = counter
	return nil
}

// otpKey scopes replay protection to one authenticator key of one user, so
// enrolling a new key is not blocked by codes used with the old one.
func otpKey(userID, secret string) string {
	return userID + "|" + secret
}

// ── Sessions ──

// ValidateToken verifies a session token and returns its user.
func (m *Manager) ValidateToken(token string) (Identity, bool) {
	if !m.enabled {
		return Anonymous, true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, u, ok := m.parseLocked(token)
	if !ok {
		return Identity{}, false
	}
	return identity(u), true
}

// Refresh re-issues a valid token that is older than a day, so active users
// never hit the expiry.
func (m *Manager) Refresh(token string) (string, bool) {
	if !m.enabled {
		return "", false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, u, ok := m.parseLocked(token)
	if !ok || time.Since(time.Unix(s.Issued, 0)) < refreshAfter {
		return "", false
	}
	m.revoked[s.ID] = s.Expires
	return m.newSessionLocked(u), true
}

// Logout revokes a session token.
func (m *Manager) Logout(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, _, ok := m.parseLocked(token); ok {
		m.revoked[s.ID] = s.Expires
	}
}

// Token format: base64url(json session) "." base64url(HMAC-SHA256(key, payload)).
func (m *Manager) newSessionLocked(u *User) string {
	now := time.Now()
	payload, _ := json.Marshal(session{User: u.ID, Gen: u.SessionGen, ID: randomHex(12), Issued: now.Unix(), Expires: now.Add(SessionTTL).Unix()})
	p := base64.RawURLEncoding.EncodeToString(payload)
	return p + "." + base64.RawURLEncoding.EncodeToString(m.sign(p))
}

func (m *Manager) parseLocked(token string) (session, *User, bool) {
	var s session
	p, sig, ok := strings.Cut(token, ".")
	if !ok || len(m.key) == 0 {
		return s, nil, false
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, m.sign(p)) {
		return s, nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(p)
	if err != nil || json.Unmarshal(raw, &s) != nil {
		return s, nil, false
	}
	u := m.byIDLocked(s.User)
	if u == nil || u.SessionGen != s.Gen || time.Now().Unix() > s.Expires {
		return s, nil, false
	}
	if _, gone := m.revoked[s.ID]; gone {
		return s, nil, false
	}
	return s, u, true
}

func (m *Manager) sign(payload string) []byte {
	h := hmac.New(sha256.New, m.key)
	h.Write([]byte(payload))
	return h.Sum(nil)
}

// ── Login throttling ──

// Allow reports whether a client may attempt a login right now.
func (m *Manager) Allow(client string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := m.failures[client]
	if f != nil && time.Now().Before(f.lockedUntil) {
		return fmt.Errorf("too many failed attempts, try again in %s", time.Until(f.lockedUntil).Round(time.Second))
	}
	return nil
}

// Fail records a failed login attempt and locks the client after repeated failures.
func (m *Manager) Fail(client string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	f := m.failures[client]
	if f == nil || now.Sub(f.first) > failureWindow {
		f = &failure{first: now}
		m.failures[client] = f
	}
	f.count++
	if f.count >= maxFailures {
		f.lockedUntil = now.Add(lockDuration)
		f.count, f.first = 0, now
		slog.Warn("login locked after repeated failures", "client", client)
	}
}

// Succeed clears the failure counter of a client.
func (m *Manager) Succeed(client string) {
	m.mu.Lock()
	delete(m.failures, client)
	m.mu.Unlock()
}

// ── User management (administrators) ──

// ListUsers returns every account, sorted by username.
func (m *Manager) ListUsers() []UserInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]UserInfo, 0, len(m.data.Users))
	for _, u := range m.data.Users {
		out = append(out, info(u))
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Username) < strings.ToLower(out[j].Username) })
	return out
}

// GetUser returns one account.
func (m *Manager) GetUser(id string) (UserInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byIDLocked(id)
	if u == nil {
		return UserInfo{}, ErrNotFound
	}
	return info(u), nil
}

// CreateUser adds an account with a temporary password. The user picks a new
// password and enrolls an authenticator at the first sign-in.
func (m *Manager) CreateUser(username, displayName, role, password string) (UserInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	username = strings.TrimSpace(username)
	if err := checkUsername(username); err != nil {
		return UserInfo{}, err
	}
	if m.byNameLocked(username) != nil {
		return UserInfo{}, errors.New("that username is already taken")
	}
	if err := checkRole(role); err != nil {
		return UserInfo{}, err
	}
	if err := checkPassword(password); err != nil {
		return UserInfo{}, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return UserInfo{}, err
	}
	now := time.Now().UTC()
	u := &User{
		ID: randomHex(8), Username: username, DisplayName: strings.TrimSpace(displayName), Role: role,
		PasswordHash: hash, MustChangePassword: true, CreatedAt: now, UpdatedAt: now,
	}
	m.data.Users = append(m.data.Users, u)
	if err := m.save(); err != nil {
		return UserInfo{}, fmt.Errorf("save auth file: %w", err)
	}
	slog.Info("user created", "username", username, "role", role)
	return info(u), nil
}

// UserPatch holds the fields to change; nil leaves a field as it is.
type UserPatch struct {
	Username    *string `json:"username"`
	DisplayName *string `json:"display_name"`
	Role        *string `json:"role"`
}

// UpdateUser changes username, display name or role.
func (m *Manager) UpdateUser(id string, patch UserPatch) (UserInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byIDLocked(id)
	if u == nil {
		return UserInfo{}, ErrNotFound
	}
	if patch.Username != nil {
		name := strings.TrimSpace(*patch.Username)
		if err := checkUsername(name); err != nil {
			return UserInfo{}, err
		}
		if other := m.byNameLocked(name); other != nil && other != u {
			return UserInfo{}, errors.New("that username is already taken")
		}
		u.Username = name
	}
	if patch.DisplayName != nil {
		if len(*patch.DisplayName) > 64 {
			return UserInfo{}, errors.New("display name is too long")
		}
		u.DisplayName = strings.TrimSpace(*patch.DisplayName)
	}
	if patch.Role != nil && *patch.Role != u.Role {
		if err := checkRole(*patch.Role); err != nil {
			return UserInfo{}, err
		}
		if u.Role == RoleAdmin && m.adminsLocked() == 1 {
			return UserInfo{}, errors.New("keep at least one administrator")
		}
		u.Role = *patch.Role
	}
	m.touchLocked(u)
	if err := m.save(); err != nil {
		return UserInfo{}, fmt.Errorf("save auth file: %w", err)
	}
	return info(u), nil
}

// ResetPassword gives a user a temporary password to change at the next
// sign-in and ends every session of that user.
func (m *Manager) ResetPassword(id, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byIDLocked(id)
	if u == nil {
		return ErrNotFound
	}
	if err := checkPassword(password); err != nil {
		return err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	u.PasswordHash, u.MustChangePassword = hash, true
	u.SessionGen++
	m.touchLocked(u)
	slog.Info("password reset by an administrator", "username", u.Username)
	return m.save()
}

// Reset2FA removes the authenticator of a user, who enrolls a new one at the
// next sign-in. Every session of that user ends.
func (m *Manager) Reset2FA(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byIDLocked(id)
	if u == nil {
		return ErrNotFound
	}
	u.TOTPSecret = ""
	u.SessionGen++
	m.touchLocked(u)
	slog.Info("2FA reset by an administrator", "username", u.Username)
	return m.save()
}

// DeleteUser removes an account. The last administrator cannot be removed.
func (m *Manager) DeleteUser(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byIDLocked(id)
	if u == nil {
		return ErrNotFound
	}
	if u.Role == RoleAdmin && m.adminsLocked() == 1 {
		return errors.New("keep at least one administrator")
	}
	m.data.Users = deleteUser(m.data.Users, u)
	slog.Info("user deleted", "username", u.Username)
	return m.save()
}

// ── Own profile ──

// ChangePassword sets a new password after checking the current one. Other
// sessions end; the returned token replaces the caller's.
func (m *Manager) ChangePassword(id, current, password string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byIDLocked(id)
	if u == nil {
		return "", ErrNotFound
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(current)) != nil {
		return "", errors.New("the current password is wrong")
	}
	if err := checkPassword(password); err != nil {
		return "", err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return "", err
	}
	u.PasswordHash, u.MustChangePassword = hash, false
	u.SessionGen++
	m.touchLocked(u)
	if err := m.save(); err != nil {
		return "", fmt.Errorf("save auth file: %w", err)
	}
	return m.newSessionLocked(u), nil
}

// RevealTOTP returns the authenticator key after checking the password, so it
// can be added to another device.
func (m *Manager) RevealTOTP(id, password string) (secret, otpauthURL string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byIDLocked(id)
	if u == nil {
		return "", "", ErrNotFound
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", "", errors.New("the password is wrong")
	}
	if u.TOTPSecret == "" {
		return "", "", errors.New("two-factor sign-in is not set up for this account")
	}
	return u.TOTPSecret, GenerateOTPAuthURL(u.Username, u.TOTPSecret), nil
}

// SetRecovery stores the recovery question and answer of an administrator.
func (m *Manager) SetRecovery(id, password, question, answer string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byIDLocked(id)
	if u == nil {
		return ErrNotFound
	}
	if u.Role != RoleAdmin {
		return errors.New("only administrators use recovery questions, operators are reset by an administrator")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return errors.New("the password is wrong")
	}
	hash, err := recoveryHash(question, answer)
	if err != nil {
		return err
	}
	u.RecoveryQuestion, u.RecoveryAnswerHash = question, hash
	m.touchLocked(u)
	return m.save()
}

// SetRecoveryFor lets an administrator set the recovery question of another
// administrator, e.g. an account created or migrated without one. Recovery
// still needs the account's 2FA code or password besides this answer.
func (m *Manager) SetRecoveryFor(id, question, answer string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byIDLocked(id)
	if u == nil {
		return ErrNotFound
	}
	if u.Role != RoleAdmin {
		return errors.New("only administrators use recovery questions, operators are reset by an administrator")
	}
	hash, err := recoveryHash(question, answer)
	if err != nil {
		return err
	}
	u.RecoveryQuestion, u.RecoveryAnswerHash = question, hash
	m.touchLocked(u)
	slog.Info("recovery question set by an administrator", "username", u.Username)
	return m.save()
}

// ── Helpers ──

func (m *Manager) byIDLocked(id string) *User {
	for _, u := range m.data.Users {
		if u.ID == id {
			return u
		}
	}
	return nil
}

// Usernames are matched case-insensitively.
func (m *Manager) byNameLocked(name string) *User {
	name = strings.TrimSpace(name)
	for _, u := range m.data.Users {
		if strings.EqualFold(u.Username, name) {
			return u
		}
	}
	return nil
}

func (m *Manager) adminsLocked() int {
	n := 0
	for _, u := range m.data.Users {
		if u.Role == RoleAdmin {
			n++
		}
	}
	return n
}

func (m *Manager) touchLocked(u *User) {
	u.UpdatedAt = time.Now().UTC()
	if err := m.save(); err != nil {
		slog.Warn("failed to save auth file", "error", err)
	}
}

func deleteUser(users []*User, u *User) []*User {
	out := users[:0]
	for _, x := range users {
		if x != u {
			out = append(out, x)
		}
	}
	return out
}

func identity(u *User) Identity {
	name := u.DisplayName
	if name == "" {
		name = u.Username
	}
	return Identity{ID: u.ID, Username: u.Username, DisplayName: name, Role: u.Role}
}

func info(u *User) UserInfo {
	return UserInfo{
		ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Role: u.Role,
		Has2FA: u.TOTPSecret != "", MustChangePassword: u.MustChangePassword,
		HasRecovery: u.RecoveryAnswerHash != "", RecoveryQuestion: u.RecoveryQuestion,
		CreatedAt: u.CreatedAt, LastLoginAt: u.LastLoginAt,
	}
}

func checkUsername(s string) error {
	if !usernameRe.MatchString(s) {
		return errors.New("username must be 2-32 characters: letters, digits, dot, dash, underscore or @")
	}
	return nil
}

// checkPassword enforces the password policy: at least MinPasswordLen
// characters mixing lowercase, uppercase, a digit and a symbol (e.g. Qawsed#1477).
func checkPassword(s string) error {
	if len(s) > 72 { // bcrypt ignores anything past 72 bytes
		return errors.New("password must be at most 72 characters")
	}
	var lower, upper, digit, symbol bool
	for _, r := range s {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		case !unicode.IsLetter(r) && !unicode.IsSpace(r):
			symbol = true
		}
	}
	if utf8.RuneCountInString(s) < MinPasswordLen || !lower || !upper || !digit || !symbol {
		return fmt.Errorf("password must be at least %d characters with a lowercase letter, an uppercase letter, a number and a symbol (e.g. Qawsed#1477)", MinPasswordLen)
	}
	return nil
}

func checkRole(r string) error {
	if r != RoleAdmin && r != RoleOperator {
		return errors.New("role must be admin or operator")
	}
	return nil
}

func recoveryHash(question, answer string) (string, error) {
	if _, ok := QuestionText(question); !ok {
		return "", errors.New("pick a recovery question")
	}
	a := normalizeAnswer(answer)
	if len(a) < 3 {
		return "", errors.New("the recovery answer must be at least 3 characters")
	}
	if len(a) > 72 {
		return "", errors.New("the recovery answer is too long")
	}
	return hashPassword(a)
}

func hashPassword(s string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(b), nil
}

var (
	dummyOnce sync.Once
	dummy     string
)

// dummyHash is compared against when the username does not exist.
func dummyHash() string {
	dummyOnce.Do(func() { dummy, _ = hashPassword(randomHex(16)) })
	return dummy
}

func (m *Manager) load() error {
	b, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &m.data)
}

func (m *Manager) save() error {
	if err := os.MkdirAll(filepath.Dir(m.filePath), 0o755); err != nil {
		return err
	}
	m.data.UpdatedAt = time.Now().UTC()
	b, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.filePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, m.filePath)
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for id, exp := range m.revoked {
			if now.Unix() > exp {
				delete(m.revoked, id)
			}
		}
		for t, p := range m.pending {
			if now.After(p.expires) {
				delete(m.pending, t)
			}
		}
		for c, f := range m.failures {
			if now.After(f.lockedUntil) && now.Sub(f.first) > failureWindow {
				delete(m.failures, c)
			}
		}
		m.mu.Unlock()
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
