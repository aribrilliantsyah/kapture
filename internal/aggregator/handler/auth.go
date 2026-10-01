package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/aribrilliantsyah/kapture/internal/auth"
)

// SessionCookie holds the signed session token. HttpOnly keeps it away from
// page scripts; the WebSocket live tail authenticates with it too.
const SessionCookie = "kapture_session"

type identityKey struct{}

// IdentityOf returns the signed-in user of a request that passed Wrap.
func IdentityOf(r *http.Request) auth.Identity {
	if id, ok := r.Context().Value(identityKey{}).(auth.Identity); ok {
		return id
	}
	return auth.Anonymous
}

func (h *Handler) registerAuth(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auth/status", h.authStatus)
	mux.HandleFunc("GET /api/v1/auth/questions", h.questions)
	mux.HandleFunc("POST /api/v1/auth/setup/init", h.setupInit)
	mux.HandleFunc("POST /api/v1/auth/setup/complete", h.setupComplete)
	mux.HandleFunc("POST /api/v1/auth/login/credentials", h.loginCredentials)
	mux.HandleFunc("POST /api/v1/auth/login/2fa", h.login2FA)
	mux.HandleFunc("POST /api/v1/auth/login/password", h.loginPassword)
	mux.HandleFunc("POST /api/v1/auth/login/enroll", h.loginEnroll)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/v1/auth/recover/start", h.recoverStart)
	mux.HandleFunc("POST /api/v1/auth/recover/verify", h.recoverVerify)
	mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
}

// Wrap guards pages and API routes when authentication is enabled.
// Unauthenticated page loads are redirected to /setup or /login; the browser
// keeps the #fragment across the redirect, so the app returns to the same view.
// User management is limited to administrators.
func (h *Handler) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if !h.authMgr.IsEnabled() || p == "/healthz" || strings.HasPrefix(p, "/api/v1/auth/") || isAsset(p) {
			next.ServeHTTP(w, r)
			return
		}
		token := ExtractToken(r)
		id, ok := h.authMgr.ValidateToken(token)
		setupDone := h.authMgr.IsSetupCompleted()

		if p == "/login" || p == "/setup" {
			switch {
			case !setupDone && p != "/setup":
				http.Redirect(w, r, "/setup", http.StatusFound)
			case setupDone && p == "/setup":
				http.Redirect(w, r, "/login", http.StatusFound)
			case ok:
				http.Redirect(w, r, safeNext(r.URL.Query().Get("next")), http.StatusFound)
			default:
				next.ServeHTTP(w, r)
			}
			return
		}

		if !ok {
			if strings.HasPrefix(p, "/api/") {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "sign in required", "auth_required": true})
				return
			}
			dest := "/setup"
			if setupDone {
				dest = "/login?next=" + url.QueryEscape(r.URL.RequestURI())
			}
			http.Redirect(w, r, dest, http.StatusFound)
			return
		}
		if strings.HasPrefix(p, "/api/v1/users") && !id.IsAdmin() {
			writeError(w, http.StatusForbidden, "only administrators can manage users")
			return
		}
		if fresh, refreshed := h.authMgr.Refresh(token); refreshed {
			setSession(w, r, fresh)
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey{}, id)))
	})
}

func isAsset(p string) bool {
	return strings.HasPrefix(p, "/js/") || strings.HasPrefix(p, "/styles/") || strings.HasPrefix(p, "/fonts/") ||
		p == "/favicon.png" || p == "/appicon.png"
}

// safeNext only allows local paths, so ?next= cannot redirect off-site.
func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/"
	}
	return next
}

func (h *Handler) authStatus(w http.ResponseWriter, r *http.Request) {
	if !h.authMgr.IsEnabled() {
		writeJSON(w, http.StatusOK, map[string]any{"auth_enabled": false, "setup_needed": false, "authenticated": true,
			"username": auth.Anonymous.Username, "user": auth.Anonymous})
		return
	}
	id, ok := h.authMgr.ValidateToken(ExtractToken(r))
	res := map[string]any{"auth_enabled": true, "setup_needed": !h.authMgr.IsSetupCompleted(), "authenticated": ok}
	if ok {
		res["username"], res["user"] = id.Username, id
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) questions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, auth.Questions)
}

func (h *Handler) setupInit(w http.ResponseWriter, _ *http.Request) {
	if !h.authMgr.IsEnabled() || h.authMgr.IsSetupCompleted() {
		writeError(w, http.StatusBadRequest, "setup is not available")
		return
	}
	secret := auth.GenerateSecret()
	otpauthURL := auth.GenerateOTPAuthURL("admin", secret)
	qr, err := qrDataURL(otpauthURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate QR code")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"secret": secret, "otpauth_url": otpauthURL, "qr_data_url": qr})
}

func (h *Handler) setupComplete(w http.ResponseWriter, r *http.Request) {
	if !h.authMgr.IsEnabled() {
		writeError(w, http.StatusBadRequest, "auth is disabled")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Secret   string `json:"secret"`
		Code     string `json:"code"`
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}
	if !decode(w, r, &req) {
		return
	}
	token, err := h.authMgr.CompleteSetup(req.Username, req.Password, req.Secret, req.Code, req.Question, req.Answer)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	setSession(w, r, token)
	writeJSON(w, http.StatusOK, map[string]any{"step": auth.StepDone})
}

// loginCredentials is the first step: username and password. The reply names
// the next step (totp, password or enroll) and carries a short-lived token.
func (h *Handler) loginCredentials(w http.ResponseWriter, r *http.Request) {
	if !h.authMgr.IsEnabled() {
		writeJSON(w, http.StatusOK, map[string]any{"step": auth.StepDone})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) || !h.allow(w, r, userKey(req.Username)) {
		return
	}
	res, err := h.authMgr.BeginLogin(req.Username, req.Password)
	h.stepReply(w, r, res, err, true, userKey(req.Username))
}

func (h *Handler) login2FA(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TempToken string `json:"temp_token"`
		Code      string `json:"code"`
	}
	if !decode(w, r, &req) || !h.allow(w, r, "") {
		return
	}
	res, err := h.authMgr.ContinueTOTP(req.TempToken, req.Code)
	h.stepReply(w, r, res, err, true, "")
}

// loginPassword sets the new password an account was asked to choose.
func (h *Handler) loginPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TempToken string `json:"temp_token"`
		Password  string `json:"password"`
	}
	if !decode(w, r, &req) || !h.allow(w, r, "") {
		return
	}
	res, err := h.authMgr.ContinuePassword(req.TempToken, req.Password)
	h.stepReply(w, r, res, err, false, "")
}

// loginEnroll saves a new authenticator once a code from it matches.
func (h *Handler) loginEnroll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TempToken string `json:"temp_token"`
		Code      string `json:"code"`
	}
	if !decode(w, r, &req) || !h.allow(w, r, "") {
		return
	}
	res, err := h.authMgr.ContinueEnroll(req.TempToken, req.Code)
	h.stepReply(w, r, res, err, true, "")
}

// login does both steps in one request and also returns the token, for scripts:
// curl -H "Authorization: Bearer <token>" ...
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !decode(w, r, &req) || !h.allow(w, r, userKey(req.Username)) {
		return
	}
	token, err := h.authMgr.Login(req.Username, req.Password, req.Code)
	if err != nil {
		h.fail(r, userKey(req.Username))
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	h.authMgr.Succeed(clientKey(r))
	setSession(w, r, token)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "token": token, "expires_in": int(auth.SessionTTL.Seconds())})
}

// recoverStart returns the recovery question of an administrator.
func (h *Handler) recoverStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	if !decode(w, r, &req) || !h.allow(w, r, userKey(req.Username)) {
		return
	}
	question, res, err := h.authMgr.BeginRecovery(req.Username)
	if err != nil {
		h.fail(r, userKey(req.Username))
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"question": question, "temp_token": res.TempToken, "username": res.Username, "expires_in": res.ExpiresIn})
}

// recoverVerify checks the answer plus the code (mode=password) or the
// password (mode=2fa) and continues with the matching reset step.
func (h *Handler) recoverVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TempToken string `json:"temp_token"`
		Answer    string `json:"answer"`
		Mode      string `json:"mode"`
		Code      string `json:"code"`
		Password  string `json:"password"`
	}
	if !decode(w, r, &req) || !h.allow(w, r, "") {
		return
	}
	res, err := h.authMgr.FinishRecovery(req.TempToken, req.Answer, req.Mode, req.Code, req.Password)
	h.stepReply(w, r, res, err, true, "")
}

// stepReply answers a sign-in step. An expired step is flagged so the page
// can go back to the password form; wrong secrets count towards the lockout.
func (h *Handler) stepReply(w http.ResponseWriter, r *http.Request, res *auth.StepResult, err error, countFailure bool, key string) {
	if errors.Is(err, auth.ErrStepExpired) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": err.Error(), "expired": true})
		return
	}
	if err != nil {
		code := http.StatusBadRequest
		if countFailure {
			h.fail(r, key)
			code = http.StatusUnauthorized
		}
		writeError(w, code, err.Error())
		return
	}
	if res.Step == auth.StepDone {
		h.authMgr.Succeed(clientKey(r))
		setSession(w, r, res.Session)
		writeJSON(w, http.StatusOK, map[string]any{"step": auth.StepDone, "username": res.Username})
		return
	}
	out := map[string]any{"step": res.Step, "temp_token": res.TempToken, "username": res.Username, "expires_in": res.ExpiresIn}
	if res.Step == auth.StepEnroll {
		qr, err := qrDataURL(res.OTPAuthURL)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate QR code")
			return
		}
		out["secret"], out["otpauth_url"], out["qr_data_url"] = res.Secret, res.OTPAuthURL, qr
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if token := ExtractToken(r); token != "" {
		h.authMgr.Logout(token)
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isHTTPS(r)})
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// allow refuses the request while the client (or the named account) is locked out.
func (h *Handler) allow(w http.ResponseWriter, r *http.Request, key string) bool {
	for _, k := range []string{clientKey(r), key} {
		if k == "" {
			continue
		}
		if err := h.authMgr.Allow(k); err != nil {
			writeError(w, http.StatusTooManyRequests, err.Error())
			return false
		}
	}
	return true
}

func (h *Handler) fail(r *http.Request, key string) {
	h.authMgr.Fail(clientKey(r))
	if key != "" {
		h.authMgr.Fail(key)
	}
}

// userKey throttles guesses against one account from many addresses.
func userKey(username string) string {
	if u := strings.ToLower(strings.TrimSpace(username)); u != "" {
		return "user:" + u
	}
	return ""
}

func qrDataURL(otpauthURL string) (string, error) {
	png, err := auth.GenerateQRCodePNG(otpauthURL)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

func setSession(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.SessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(r),
	})
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// clientKey identifies a client for login throttling (first X-Forwarded-For
// hop behind an ingress, else the socket address).
func clientKey(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		first, _, _ := strings.Cut(fwd, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ExtractToken reads the session from the Authorization header or the cookie.
func ExtractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie(SessionCookie); err == nil {
		return c.Value
	}
	return ""
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}
