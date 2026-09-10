package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

// GenerateSecret creates a 20-byte random secret encoded as unpadded base32 (32 chars).
func GenerateSecret() string {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-seeded random if system crypto rand fails
		binary.BigEndian.PutUint64(b, uint64(time.Now().UnixNano()))
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

// GenerateOTPAuthURL creates a standard RFC 6238 otpauth URI for Google Authenticator.
func GenerateOTPAuthURL(username, secret string) string {
	issuer := "Kapture"
	label := fmt.Sprintf("%s:%s", issuer, username)
	return fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		url.PathEscape(label),
		secret,
		url.QueryEscape(issuer),
	)
}

// GenerateQRCodePNG generates a PNG QR code for the given otpauth URL.
func GenerateQRCodePNG(otpauthURL string) ([]byte, error) {
	return qrcode.Encode(otpauthURL, qrcode.Medium, 256)
}

// ComputeHOTP computes a 6-digit HOTP code for a given base32 secret and counter.
func ComputeHOTP(secret string, counter uint64) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", fmt.Errorf("invalid base32 secret: %w", err)
	}

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	h := hmac.New(sha1.New, key)
	h.Write(buf)
	sum := h.Sum(nil)

	offset := sum[19] & 0x0f
	binCode := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]&0xff) << 16) |
		(uint32(sum[offset+2]&0xff) << 8) |
		(uint32(sum[offset+3] & 0xff))

	code := binCode % 1000000
	return fmt.Sprintf("%06d", code), nil
}

// VerifyTOTP validates a 6-digit OTP code against the secret within a +/- 30s window.
func VerifyTOTP(secret, inputCode string) bool {
	cleanCode := strings.TrimSpace(inputCode)
	cleanCode = strings.ReplaceAll(cleanCode, "-", "")
	cleanCode = strings.ReplaceAll(cleanCode, " ", "")

	if len(cleanCode) != 6 {
		return false
	}

	now := time.Now().Unix() / 30

	// Check time windows: -30s, current, +30s to tolerate minor clock drift
	for _, offset := range []int64{-1, 0, 1} {
		expected, err := ComputeHOTP(secret, uint64(now+offset))
		if err != nil {
			return false
		}
		if expected == cleanCode {
			return true
		}
	}

	return false
}
