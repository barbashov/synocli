package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"
)

type GlobalOptions struct {
	User            string
	Password        string
	PasswordStdin   bool
	CredentialsFile string
	InsecureTLS     bool
	Timeout         time.Duration
	JSON            bool
	Debug           bool
}

func ValidateEndpoint(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("endpoint must use http or https: %q", raw)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("endpoint must include host: %q", raw)
	}
	if u.Path != "" && u.Path != "/" {
		return nil, errors.New("endpoint must not contain path")
	}
	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u, nil
}

func (o *GlobalOptions) ResolvePassword(stdin io.Reader) error {
	if o.CredentialsFile != "" {
		if o.User != "" || o.Password != "" || o.PasswordStdin {
			return errors.New("use --credentials-file without --user, --password, or --password-stdin")
		}
		if err := o.loadCredentialsFile(); err != nil {
			return err
		}
	}
	if o.PasswordStdin {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("read password from stdin: %w", err)
		}
		o.Password = strings.TrimRight(string(b), "\r\n")
	}
	if o.User == "" {
		return errors.New("--user is required")
	}
	if o.Password == "" {
		return errors.New("password is required via --password or --password-stdin")
	}
	return nil
}

func (o *GlobalOptions) loadCredentialsFile() error {
	b, err := os.ReadFile(o.CredentialsFile)
	if err != nil {
		return fmt.Errorf("read credentials file: %w", err)
	}
	s := bufio.NewScanner(strings.NewReader(string(b)))
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid credentials file format at line %d", lineNo)
		}
		key := strings.ToLower(strings.TrimSpace(k))
		value := strings.TrimSpace(v)
		switch key {
		case "user":
			o.User = value
		case "password":
			o.Password = value
		default:
			// Ignore unknown keys for forward compatibility.
		}
	}
	if err := s.Err(); err != nil {
		return fmt.Errorf("read credentials file: %w", err)
	}
	return nil
}
