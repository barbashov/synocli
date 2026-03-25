package httpclient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"time"

	"synocli/internal/redact"
)

type Options struct {
	InsecureTLS bool
	Timeout     time.Duration
	Debug       bool
	DebugOut    io.Writer
}

func New(opts Options) (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: opts.InsecureTLS}}
	var rt http.RoundTripper = tr
	if opts.Debug {
		if opts.DebugOut == nil {
			opts.DebugOut = io.Discard
		}
		rt = &debugRoundTripper{next: tr, out: opts.DebugOut}
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{Transport: rt, Timeout: timeout, Jar: jar}, nil
}

type debugRoundTripper struct {
	next http.RoundTripper
	out  io.Writer
}

func (d *debugRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	baseURL := req.URL.Scheme + "://" + req.URL.Host + req.URL.Path
	_, _ = fmt.Fprintf(d.out, "[debug] -> %s %s", req.Method, baseURL)
	if len(req.URL.Query()) > 0 {
		keys := make([]string, 0, len(req.URL.Query()))
		for k := range req.URL.Query() {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		pairs := make([]string, 0, len(keys))
		for _, k := range keys {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, redact.Value(k, req.URL.Query().Get(k))))
		}
		_, _ = fmt.Fprintf(d.out, "?%s", strings.Join(pairs, "&"))
	}
	_, _ = fmt.Fprintln(d.out)
	for k, vals := range req.Header {
		for _, v := range vals {
			_, _ = fmt.Fprintf(d.out, "[debug]   header %s=%s\n", k, redact.HeaderValue(k, v))
		}
	}
	d.logRequestBody(req)
	resp, err := d.next.RoundTrip(req)
	dur := time.Since(start)
	if err != nil {
		_, _ = fmt.Fprintf(d.out, "[debug] <- error after %s: %v\n", dur, err)
		return nil, err
	}
	_, _ = fmt.Fprintf(d.out, "[debug] <- status=%s after %s\n", resp.Status, dur)
	d.logResponseBody(resp)
	if req.URL != nil {
		hostURL := &url.URL{Scheme: req.URL.Scheme, Host: req.URL.Host}
		for _, c := range resp.Cookies() {
			_, _ = fmt.Fprintf(d.out, "[debug]   set-cookie %s=%s\n", c.Name, redact.HeaderValue("Set-Cookie", c.Value))
		}
		for _, c := range req.Cookies() {
			_ = hostURL
			_, _ = fmt.Fprintf(d.out, "[debug]   sent-cookie %s=%s\n", c.Name, redact.HeaderValue("Cookie", c.Value))
		}
	}
	return resp, nil
}

func (d *debugRoundTripper) logRequestBody(req *http.Request) {
	if req.Body == nil {
		return
	}
	if req.GetBody == nil {
		_, _ = fmt.Fprintln(d.out, "[debug]   body=<stream unavailable>")
		return
	}
	body, err := cloneBody(req.GetBody)
	if err != nil {
		_, _ = fmt.Fprintf(d.out, "[debug]   body=<read error: %v>\n", err)
		return
	}
	d.logBody("request", req.Header.Get("Content-Type"), body)
}

func (d *debugRoundTripper) logResponseBody(resp *http.Response) {
	if resp.Body == nil {
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		_, _ = fmt.Fprintf(d.out, "[debug]   response_body=<read error: %v>\n", err)
		return
	}
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	d.logBody("response", resp.Header.Get("Content-Type"), body)
}

func (d *debugRoundTripper) logBody(kind, contentType string, body []byte) {
	_, _ = fmt.Fprintf(d.out, "[debug]   %s_body_bytes=%d\n", kind, len(body))
	if len(body) == 0 {
		return
	}
	summary := summarizeBody(contentType, body)
	for _, line := range strings.Split(summary, "\n") {
		if line == "" {
			continue
		}
		_, _ = fmt.Fprintf(d.out, "[debug]   %s_body %s\n", kind, line)
	}
}

func cloneBody(getBody func() (io.ReadCloser, error)) ([]byte, error) {
	rc, err := getBody()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

func summarizeBody(contentType string, body []byte) string {
	if strings.Contains(strings.ToLower(contentType), "multipart/form-data") {
		return summarizeMultipart(contentType, body)
	}
	if strings.Contains(strings.ToLower(contentType), "application/x-www-form-urlencoded") {
		return summarizeFormURLEncoded(body)
	}
	if strings.Contains(strings.ToLower(contentType), "application/json") {
		return summarizeJSON(body)
	}
	if len(body) > 4096 {
		return "<omitted: non-JSON body larger than 4KiB>"
	}
	return string(body)
}

func summarizeFormURLEncoded(body []byte) string {
	vals, err := url.ParseQuery(string(body))
	if err != nil {
		return fmt.Sprintf("<parse error: %v>", err)
	}
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, redact.Value(k, vals.Get(k))))
	}
	return strings.Join(parts, "&")
}

func summarizeJSON(body []byte) string {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		if len(body) > 4096 {
			return "<invalid JSON; omitted: body larger than 4KiB>"
		}
		return string(body)
	}
	redactJSON(v)
	out, err := json.Marshal(v)
	if err != nil {
		return "<json marshal error>"
	}
	return string(out)
}

func redactJSON(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, value := range t {
			if s, ok := value.(string); ok {
				t[k] = redact.Value(k, s)
				continue
			}
			redactJSON(value)
		}
	case []any:
		for _, it := range t {
			redactJSON(it)
		}
	}
}

func summarizeMultipart(contentType string, body []byte) string {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Sprintf("<multipart parse media type error: %v>", err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		return "<multipart boundary missing>"
	}
	mr := multipart.NewReader(bytes.NewReader(body), boundary)
	var lines []string
	const maxTextPart = 2048
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			lines = append(lines, fmt.Sprintf("<multipart read error: %v>", err))
			break
		}
		name := part.FormName()
		filename := part.FileName()
		pct := part.Header.Get("Content-Type")
		if filename != "" {
			size, _ := io.Copy(io.Discard, part)
			lines = append(lines, fmt.Sprintf("part name=%q file=%q content_type=%q bytes=%d", name, filename, pct, size))
			continue
		}
		buf, _ := io.ReadAll(io.LimitReader(part, maxTextPart+1))
		val := string(buf)
		if len(buf) > maxTextPart {
			val = val[:maxTextPart] + "...<truncated>"
		}
		val = redact.Value(name, val)
		lines = append(lines, fmt.Sprintf("part name=%q value=%q", name, val))
	}
	if len(lines) == 0 {
		return "<multipart with no parts>"
	}
	return strings.Join(lines, "\n")
}
