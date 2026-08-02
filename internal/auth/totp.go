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
)

func NewTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}
func TOTPURI(secret, username string) string {
	label := url.PathEscape("AnPanel:" + username)
	q := url.Values{"secret": {secret}, "issuer": {"AnPanel"}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + label + "?" + q.Encode()
}
func VerifyTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	for d := -1; d <= 1; d++ {
		if totpCode(secret, now.Add(time.Duration(d)*30*time.Second)) == code {
			return true
		}
	}
	return false
}
func totpCode(secret string, t time.Time) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	counter := uint64(t.Unix() / 30)
	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, counter)
	h := hmac.New(sha1.New, key)
	_, _ = h.Write(msg)
	sum := h.Sum(nil)
	off := sum[len(sum)-1] & 15
	num := (uint32(sum[off])&0x7f)<<24 | uint32(sum[off+1])<<16 | uint32(sum[off+2])<<8 | uint32(sum[off+3])
	v := num % 1_000_000
	return fmt.Sprintf("%06d", v)
}
