package redact

import "strings"

var secretKeys = map[string]struct{}{
	"password":      {},
	"passwd":        {},
	"account":       {},
	"sid":           {},
	"_sid":          {},
	"token":         {},
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
	if strings.EqualFold(key, "authorization") || strings.EqualFold(key, "cookie") || strings.EqualFold(key, "set-cookie") {
		return "<redacted>"
	}
	return value
}
