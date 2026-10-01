package handler

import (
	"errors"
	"net/http"

	"github.com/aribrilliantsyah/kapture/internal/auth"
)

// User management (/api/v1/users, administrators only; Wrap enforces the
// role) and the signed-in user's own profile (/api/v1/profile).
func (h *Handler) registerUsers(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/users", h.listUsers)
	mux.HandleFunc("POST /api/v1/users", h.createUser)
	mux.HandleFunc("PATCH /api/v1/users/{id}", h.updateUser)
	mux.HandleFunc("POST /api/v1/users/{id}/password", h.resetUserPassword)
	mux.HandleFunc("POST /api/v1/users/{id}/2fa/reset", h.resetUser2FA)
	mux.HandleFunc("PUT /api/v1/users/{id}/recovery", h.setUserRecovery)
	mux.HandleFunc("DELETE /api/v1/users/{id}", h.deleteUser)

	mux.HandleFunc("GET /api/v1/profile", h.profile)
	mux.HandleFunc("PATCH /api/v1/profile", h.updateProfile)
	mux.HandleFunc("POST /api/v1/profile/password", h.changePassword)
	mux.HandleFunc("POST /api/v1/profile/2fa", h.revealTOTP)
	mux.HandleFunc("PUT /api/v1/profile/recovery", h.setRecovery)
}

// authOn rejects account endpoints when authentication is disabled.
func (h *Handler) authOn(w http.ResponseWriter) bool {
	if !h.authMgr.IsEnabled() {
		writeError(w, http.StatusBadRequest, "authentication is disabled (KAPTURE_AUTH_ENABLED=false)")
		return false
	}
	return true
}

func (h *Handler) listUsers(w http.ResponseWriter, _ *http.Request) {
	if !h.authOn(w) {
		return
	}
	writeJSON(w, http.StatusOK, h.authMgr.ListUsers())
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Password    string `json:"password"`
	}
	if !h.authOn(w) || !decode(w, r, &req) {
		return
	}
	u, err := h.authMgr.CreateUser(req.Username, req.DisplayName, req.Role, req.Password)
	userReply(w, u, err)
}

func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	var patch auth.UserPatch
	if !h.authOn(w) || !decode(w, r, &patch) {
		return
	}
	u, err := h.authMgr.UpdateUser(r.PathValue("id"), patch)
	userReply(w, u, err)
}

func (h *Handler) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if !h.authOn(w) || !decode(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	if id == IdentityOf(r).ID {
		writeError(w, http.StatusBadRequest, "change your own password from your profile")
		return
	}
	okReply(w, h.authMgr.ResetPassword(id, req.Password))
}

func (h *Handler) resetUser2FA(w http.ResponseWriter, r *http.Request) {
	if !h.authOn(w) {
		return
	}
	id := r.PathValue("id")
	if id == IdentityOf(r).ID {
		writeError(w, http.StatusBadRequest, "you cannot reset your own 2FA here, use account recovery on the sign-in page")
		return
	}
	okReply(w, h.authMgr.Reset2FA(id))
}

// setUserRecovery sets another administrator's recovery question. The own
// question goes through /profile/recovery, which asks for the password.
func (h *Handler) setUserRecovery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}
	if !h.authOn(w) || !decode(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	if id == IdentityOf(r).ID {
		writeError(w, http.StatusBadRequest, "set your own recovery question from your profile")
		return
	}
	okReply(w, h.authMgr.SetRecoveryFor(id, req.Question, req.Answer))
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	if !h.authOn(w) {
		return
	}
	id := r.PathValue("id")
	if id == IdentityOf(r).ID {
		writeError(w, http.StatusBadRequest, "you cannot delete your own account")
		return
	}
	okReply(w, h.authMgr.DeleteUser(id))
}

func (h *Handler) profile(w http.ResponseWriter, r *http.Request) {
	if !h.authOn(w) {
		return
	}
	u, err := h.authMgr.GetUser(IdentityOf(r).ID)
	if err != nil {
		userReply(w, u, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "questions": auth.Questions})
}

// updateProfile changes the own username and display name (never the role).
func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    *string `json:"username"`
		DisplayName *string `json:"display_name"`
	}
	if !h.authOn(w) || !decode(w, r, &req) {
		return
	}
	u, err := h.authMgr.UpdateUser(IdentityOf(r).ID, auth.UserPatch{Username: req.Username, DisplayName: req.DisplayName})
	userReply(w, u, err)
}

// changePassword ends the other sessions and renews the caller's cookie.
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Current  string `json:"current"`
		Password string `json:"password"`
	}
	if !h.authOn(w) || !decode(w, r, &req) || !h.allow(w, r, "") {
		return
	}
	token, err := h.authMgr.ChangePassword(IdentityOf(r).ID, req.Current, req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	setSession(w, r, token)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// revealTOTP shows the authenticator key again (password required) so it
// can be added to a new phone.
func (h *Handler) revealTOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if !h.authOn(w) || !decode(w, r, &req) || !h.allow(w, r, "") {
		return
	}
	secret, otpauthURL, err := h.authMgr.RevealTOTP(IdentityOf(r).ID, req.Password)
	if err != nil {
		h.fail(r, "")
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	qr, err := qrDataURL(otpauthURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate QR code")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"secret": secret, "otpauth_url": otpauthURL, "qr_data_url": qr})
}

func (h *Handler) setRecovery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}
	if !h.authOn(w) || !decode(w, r, &req) || !h.allow(w, r, "") {
		return
	}
	okReply(w, h.authMgr.SetRecovery(IdentityOf(r).ID, req.Password, req.Question, req.Answer))
}

func userReply(w http.ResponseWriter, u auth.UserInfo, err error) {
	switch {
	case errors.Is(err, auth.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case err != nil:
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeJSON(w, http.StatusOK, u)
	}
}

func okReply(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case err != nil:
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	}
}
