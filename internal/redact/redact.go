package redact

import (
	"errors"
	"net/url"
	"strings"
)

var secretKeys = map[string]struct{}{
	"password":      {},
	"passwd":        {},
	"passphrase":    {},
	"account":       {},
	"sid":           {},
	"_sid":          {},
	"token":         {},
	"secret":        {},
	"key":           {},
	"api_key":       {},
	"apikey":        {},
	"authorization": {},
	"cookie":        {},
}

// secretSuffixes catches namespaced variants like extract_password or
// unlock_passphrase that are not worth enumerating exactly.
var secretSuffixes = []string{"password", "passwd", "passphrase", "token", "secret"}

func isSecretKey(key string) bool {
	lower := strings.ToLower(key)
	if _, ok := secretKeys[lower]; ok {
		return true
	}
	for _, suffix := range secretSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func Value(key, value string) string {
	if isSecretKey(key) {
		return "<redacted>"
	}
	return value
}

func HeaderValue(key, value string) string {
	lower := strings.ToLower(key)
	switch lower {
	case "authorization", "proxy-authorization", "cookie", "set-cookie":
		return "<redacted>"
	}
	// Catch custom auth-bearing headers (e.g. X-SYNO-TOKEN, X-Api-Key).
	if strings.Contains(lower, "token") || strings.Contains(lower, "auth") || strings.Contains(lower, "secret") {
		return "<redacted>"
	}
	return value
}

// URLString redacts secret query values and userinfo in a raw URL string.
// If the string does not parse as a URL, everything after "?" is dropped.
func URLString(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		if i := strings.IndexByte(raw, '?'); i >= 0 {
			return raw[:i] + "?<redacted>"
		}
		return raw
	}
	if u.User != nil {
		u.User = url.User("<redacted>")
	}
	if u.RawQuery != "" {
		q := u.Query()
		for k, vals := range q {
			for i := range vals {
				vals[i] = Value(k, vals[i])
			}
			q[k] = vals
		}
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// Error rewrites err so that any *url.Error in its chain no longer exposes
// credentials or session tokens carried in the request URL's query string.
// The underlying cause stays reachable via errors.Is/As; the url.Error layer
// itself is replaced.
func Error(err error) error {
	if err == nil {
		return nil
	}
	var ue *url.Error
	if !errors.As(err, &ue) {
		return err
	}
	return &urlError{op: ue.Op, url: URLString(ue.URL), err: ue.Err}
}

type urlError struct {
	op  string
	url string
	err error
}

func (e *urlError) Error() string {
	return e.op + " " + e.url + ": " + e.err.Error()
}

func (e *urlError) Unwrap() error { return e.err }

func (e *urlError) Timeout() bool {
	t, ok := e.err.(interface{ Timeout() bool })
	return ok && t.Timeout()
}
