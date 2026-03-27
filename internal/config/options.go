package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type GlobalOptions struct {
	Endpoint        string
	User            string
	Password        string
	PasswordStdin   bool
	CredentialsFile string
	ConfigPath      string
	InsecureTLS     bool
	Timeout         time.Duration
	JSON            bool
	Debug           bool
	ReuseSession    bool
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

func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".synocli", "config"), nil
}

func LoadConfigFile(path string, required bool) (GlobalOptions, error) {
	if strings.TrimSpace(path) == "" {
		return GlobalOptions{}, errors.New("config path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !required {
			return GlobalOptions{}, nil
		}
		return GlobalOptions{}, fmt.Errorf("read config file: %w", err)
	}
	if mode := info.Mode().Perm(); mode&0077 != 0 {
		return GlobalOptions{}, fmt.Errorf("config file %s has too open permissions %04o; run: chmod 600 %s", path, mode, path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return GlobalOptions{}, fmt.Errorf("read config file: %w", err)
	}
	return ParseConfigFile(string(b))
}

func ParseConfigFile(content string) (GlobalOptions, error) {
	var out GlobalOptions
	s := bufio.NewScanner(strings.NewReader(content))
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return GlobalOptions{}, fmt.Errorf("invalid config format at line %d", lineNo)
		}
		key := strings.ToLower(strings.TrimSpace(k))
		value := strings.TrimSpace(v)
		switch key {
		case "endpoint":
			out.Endpoint = value
		case "user":
			out.User = value
		case "password":
			out.Password = value
		case "insecure_tls":
			if value == "" {
				continue
			}
			b, err := strconv.ParseBool(value)
			if err != nil {
				return GlobalOptions{}, fmt.Errorf("invalid config value for insecure_tls at line %d: %w", lineNo, err)
			}
			out.InsecureTLS = b
		case "timeout":
			if value == "" {
				continue
			}
			d, err := time.ParseDuration(value)
			if err != nil {
				return GlobalOptions{}, fmt.Errorf("invalid config value for timeout at line %d: %w", lineNo, err)
			}
			out.Timeout = d
		case "reuse_session":
			if value == "" {
				continue
			}
			b, err := strconv.ParseBool(value)
			if err != nil {
				return GlobalOptions{}, fmt.Errorf("invalid config value for reuse_session at line %d: %w", lineNo, err)
			}
			out.ReuseSession = b
		default:
			return GlobalOptions{}, fmt.Errorf("unknown config key %q at line %d", key, lineNo)
		}
	}
	if err := s.Err(); err != nil {
		return GlobalOptions{}, fmt.Errorf("read config file: %w", err)
	}
	return out, nil
}

func (o *GlobalOptions) loadCredentialsFile() error {
	info, err := os.Stat(o.CredentialsFile)
	if err != nil {
		return fmt.Errorf("read credentials file: %w", err)
	}
	if mode := info.Mode().Perm(); mode&0077 != 0 {
		return fmt.Errorf("credentials file %s has too open permissions %04o; run: chmod 600 %s", o.CredentialsFile, mode, o.CredentialsFile)
	}
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
