// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
)

// SetRequestCookie adds a cookie to the given request. It is useful for
// constructing authenticated test requests.
func SetRequestCookie(r *http.Request, name, value string) {
	r.AddCookie(&http.Cookie{
		Name:  name,
		Value: value,
	})
}

// GetResponseCookie retrieves the first cookie with the given name from the
// response headers. It returns nil if no such cookie is found.
func GetResponseCookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// ExtractCookieValues returns a map of cookie names to values from the
// response's Cookie header(s). If multiple cookies have the same name,
// the last one encountered is used.
func ExtractCookieValues(resp *http.Response) map[string]string {
	cookies := make(map[string]string)
	for _, c := range resp.Cookies() {
		cookies[c.Name] = c.Value
	}
	return cookies
}

// SetCookieOnResponse adds a cookie to the http.ResponseWriter with the given
// attributes. This is a convenience for setting cookies in test handlers.
func SetCookieOnResponse(w http.ResponseWriter, name, value string, maxAge int, httpOnly, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: httpOnly,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// SignCookieValue creates a signed cookie value using HMAC‑SHA256.
// The format is "value|signature". This is a basic implementation for testing;
// for production use a more robust scheme.
func SignCookieValue(value, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(value))
	signature := base64.URLEncoding.EncodeToString(h.Sum(nil))
	return value + "|" + signature
}

// VerifySignedCookieValue verifies a signed cookie value produced by SignCookieValue.
// It returns the original value if the signature is valid, otherwise an empty string.
func VerifySignedCookieValue(signedValue, secret string) string {
	parts := strings.SplitN(signedValue, "|", 2)
	if len(parts) != 2 {
		return ""
	}
	value, signature := parts[0], parts[1]
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(value))
	expected := base64.URLEncoding.EncodeToString(h.Sum(nil))
	if signature != expected {
		return ""
	}
	return value
}