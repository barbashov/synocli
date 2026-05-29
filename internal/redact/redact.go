package redact

import "strings"

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

func Value(key, value string) string {
	if _, ok := secretKeys[strings.ToLower(key)]; ok {
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
