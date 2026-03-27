package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultOwner = "barbashov"
	DefaultRepo  = "synocli"

	successInterval = 24 * time.Hour
	failureInterval = 6 * time.Hour
)

var semverRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	Owner   string
	Repo    string
	BaseURL string
	HTTP    HTTPDoer
	Now     func() time.Time
}

type Release struct {
	Tag    string
	Assets map[string]string
}

type State struct {
	LastAttemptAt time.Time
	LastSuccessAt time.Time
	LatestVersion string
}

type CheckResult struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	Checked         bool
}

type ApplyResult struct {
	CurrentVersion string
	LatestVersion  string
	Updated        bool
	BinaryPath     string
}

type stateFile struct {
	LastAttemptAt string `json:"last_attempt_at,omitempty"`
	LastSuccessAt string `json:"last_success_at,omitempty"`
	LatestVersion string `json:"latest_version,omitempty"`
}

func NewClient(httpClient HTTPDoer) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		Owner:   DefaultOwner,
		Repo:    DefaultRepo,
		BaseURL: "https://api.github.com",
		HTTP:    httpClient,
		Now:     time.Now,
	}
}

func (c *Client) now() time.Time {
	if c.Now == nil {
		return time.Now().UTC()
	}
	return c.Now().UTC()
}

func (c *Client) apiBaseURL() string {
	if strings.TrimSpace(c.BaseURL) == "" {
		return "https://api.github.com"
	}
	return strings.TrimRight(c.BaseURL, "/")
}

func StatePathFromConfig(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "update-check.json")
}

func LoadState(statePath string) (State, error) {
	b, err := os.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, nil
		}
		return State{}, err
	}
	var raw stateFile
	if err := json.Unmarshal(b, &raw); err != nil {
		return State{}, fmt.Errorf("decode update state: %w", err)
	}
	st := State{LatestVersion: strings.TrimSpace(raw.LatestVersion)}
	if raw.LastAttemptAt != "" {
		t, err := time.Parse(time.RFC3339, raw.LastAttemptAt)
		if err != nil {
			return State{}, fmt.Errorf("parse last_attempt_at: %w", err)
		}
		st.LastAttemptAt = t.UTC()
	}
	if raw.LastSuccessAt != "" {
		t, err := time.Parse(time.RFC3339, raw.LastSuccessAt)
		if err != nil {
			return State{}, fmt.Errorf("parse last_success_at: %w", err)
		}
		st.LastSuccessAt = t.UTC()
	}
	return st, nil
}

func WriteState(statePath string, st State) error {
	if err := os.MkdirAll(filepath.Dir(statePath), 0o700); err != nil {
		return fmt.Errorf("create update state dir: %w", err)
	}
	raw := stateFile{LatestVersion: st.LatestVersion}
	if !st.LastAttemptAt.IsZero() {
		raw.LastAttemptAt = st.LastAttemptAt.UTC().Format(time.RFC3339)
	}
	if !st.LastSuccessAt.IsZero() {
		raw.LastSuccessAt = st.LastSuccessAt.UTC().Format(time.RFC3339)
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("encode update state: %w", err)
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(statePath), ".update-check-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp update state: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp update state: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp update state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp update state: %w", err)
	}
	if err := os.Rename(tmpPath, statePath); err != nil {
		return fmt.Errorf("replace update state: %w", err)
	}
	return nil
}

func ShouldBackgroundCheck(now time.Time, st State) bool {
	if st.LastAttemptAt.IsZero() {
		return true
	}
	now = now.UTC()
	if st.LastSuccessAt.IsZero() || st.LastSuccessAt.Before(st.LastAttemptAt) {
		return now.Sub(st.LastAttemptAt) >= failureInterval
	}
	return now.Sub(st.LastAttemptAt) >= successInterval
}

func (c *Client) CheckForUpdate(ctx context.Context, currentVersion, statePath string, force bool) (CheckResult, error) {
	res := CheckResult{CurrentVersion: strings.TrimSpace(currentVersion)}
	st, err := LoadState(statePath)
	if err != nil {
		return res, err
	}
	if !force && !ShouldBackgroundCheck(c.now(), st) {
		res.LatestVersion = strings.TrimSpace(st.LatestVersion)
		res.UpdateAvailable = IsNewerVersion(res.LatestVersion, res.CurrentVersion)
		return res, nil
	}
	rel, err := c.FetchLatestRelease(ctx)
	st.LastAttemptAt = c.now()
	if err != nil {
		_ = WriteState(statePath, st)
		res.Checked = true
		res.LatestVersion = strings.TrimSpace(st.LatestVersion)
		res.UpdateAvailable = IsNewerVersion(res.LatestVersion, res.CurrentVersion)
		return res, err
	}
	st.LastSuccessAt = st.LastAttemptAt
	st.LatestVersion = rel.Tag
	if writeErr := WriteState(statePath, st); writeErr != nil {
		return res, writeErr
	}
	res.Checked = true
	res.LatestVersion = rel.Tag
	res.UpdateAvailable = IsNewerVersion(rel.Tag, res.CurrentVersion)
	return res, nil
}

func IsNewerVersion(latest, current string) bool {
	latest = strings.TrimSpace(latest)
	current = strings.TrimSpace(current)
	if latest == "" {
		return false
	}
	lm, lok := parseSemver(latest)
	if !lok {
		return false
	}
	cm, cok := parseSemver(current)
	if !cok {
		return true
	}
	if lm.major != cm.major {
		return lm.major > cm.major
	}
	if lm.minor != cm.minor {
		return lm.minor > cm.minor
	}
	return lm.patch > cm.patch
}

type semver struct {
	major int
	minor int
	patch int
}

func parseSemver(v string) (semver, bool) {
	m := semverRe.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return semver{}, false
	}
	maj, err := strconv.Atoi(m[1])
	if err != nil {
		return semver{}, false
	}
	min, err := strconv.Atoi(m[2])
	if err != nil {
		return semver{}, false
	}
	pat, err := strconv.Atoi(m[3])
	if err != nil {
		return semver{}, false
	}
	return semver{major: maj, minor: min, patch: pat}, true
}

func (c *Client) FetchLatestRelease(ctx context.Context) (Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.apiBaseURL(), c.Owner, c.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, fmt.Errorf("build releases request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "synocli")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("request latest release: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return Release{}, fmt.Errorf("latest release request failed: status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		TagName    string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Release{}, fmt.Errorf("decode latest release: %w", err)
	}
	if payload.Draft || payload.Prerelease {
		return Release{}, errors.New("latest release is not stable")
	}
	tag := strings.TrimSpace(payload.TagName)
	if tag == "" {
		return Release{}, errors.New("latest release has empty tag")
	}
	assets := make(map[string]string, len(payload.Assets))
	for _, a := range payload.Assets {
		name := strings.TrimSpace(a.Name)
		url := strings.TrimSpace(a.URL)
		if name == "" || url == "" {
			continue
		}
		assets[name] = url
	}
	return Release{Tag: tag, Assets: assets}, nil
}

func AssetName(tag, goos, goarch string) (string, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", errors.New("tag is required")
	}
	switch goos {
	case "linux", "darwin":
		if goarch != "amd64" && goarch != "arm64" {
			return "", fmt.Errorf("unsupported architecture %q", goarch)
		}
		return fmt.Sprintf("synocli_%s_%s_%s.tar.gz", tag, goos, goarch), nil
	case "windows":
		if goarch != "amd64" && goarch != "arm64" {
			return "", fmt.Errorf("unsupported architecture %q", goarch)
		}
		return fmt.Sprintf("synocli_%s_%s_%s.zip", tag, goos, goarch), nil
	default:
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
}

func (c *Client) ApplyUpdate(ctx context.Context, release Release, currentVersion, executablePath, goos, goarch string) (ApplyResult, error) {
	res := ApplyResult{
		CurrentVersion: strings.TrimSpace(currentVersion),
		LatestVersion:  strings.TrimSpace(release.Tag),
		BinaryPath:     executablePath,
	}
	if !IsNewerVersion(release.Tag, currentVersion) {
		return res, nil
	}
	if goos == "windows" && runtime.GOOS == "windows" {
		return res, errors.New("native Windows self-update is not supported; use WSL2 or install script")
	}
	archiveName, err := AssetName(release.Tag, goos, goarch)
	if err != nil {
		return res, err
	}
	archiveURL := release.Assets[archiveName]
	if archiveURL == "" {
		return res, fmt.Errorf("release asset not found: %s", archiveName)
	}
	sumsURL := release.Assets["SHA256SUMS"]
	if sumsURL == "" {
		return res, errors.New("release asset not found: SHA256SUMS")
	}

	archiveBytes, err := c.downloadAsset(ctx, archiveURL)
	if err != nil {
		return res, fmt.Errorf("download %s: %w", archiveName, err)
	}
	sumsBytes, err := c.downloadAsset(ctx, sumsURL)
	if err != nil {
		return res, fmt.Errorf("download SHA256SUMS: %w", err)
	}
	if err := verifyChecksum(archiveName, archiveBytes, sumsBytes); err != nil {
		return res, err
	}
	binBytes, err := extractBinary(archiveName, archiveBytes)
	if err != nil {
		return res, err
	}
	if err := replaceExecutable(executablePath, binBytes); err != nil {
		return res, err
	}
	res.Updated = true
	return res, nil
}

func (c *Client) downloadAsset(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build download request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "synocli")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("download failed: status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read download body: %w", err)
	}
	return b, nil
}

func verifyChecksum(archiveName string, archiveBytes, sumsBytes []byte) error {
	hash := sha256.Sum256(archiveBytes)
	expected := ""
	for _, line := range strings.Split(string(sumsBytes), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == archiveName {
			expected = fields[0]
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("checksum for %s not found in SHA256SUMS", archiveName)
	}
	actual := hex.EncodeToString(hash[:])
	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("checksum mismatch for %s", archiveName)
	}
	return nil
}

func extractBinary(archiveName string, archiveBytes []byte) ([]byte, error) {
	if strings.HasSuffix(archiveName, ".tar.gz") {
		return extractBinaryFromTarGz(archiveBytes)
	}
	if strings.HasSuffix(archiveName, ".zip") {
		return extractBinaryFromZip(archiveBytes)
	}
	return nil, fmt.Errorf("unsupported archive format for %s", archiveName)
}

func extractBinaryFromTarGz(archiveBytes []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archiveBytes))
	if err != nil {
		return nil, fmt.Errorf("open tar.gz: %w", err)
	}
	defer func() {
		_ = gz.Close()
	}()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar entry: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := path.Base(hdr.Name)
		if name != "synocli" {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read binary from tar.gz: %w", err)
		}
		if len(b) == 0 {
			return nil, errors.New("binary in archive is empty")
		}
		return b, nil
	}
	return nil, errors.New("synocli binary not found in archive")
}

func extractBinaryFromZip(archiveBytes []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	for _, f := range zr.File {
		if path.Base(f.Name) != "synocli.exe" && path.Base(f.Name) != "synocli" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open binary from zip: %w", err)
		}
		b, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read binary from zip: %w", readErr)
		}
		if len(b) == 0 {
			return nil, errors.New("binary in archive is empty")
		}
		return b, nil
	}
	return nil, errors.New("synocli binary not found in archive")
}

func replaceExecutable(executablePath string, binary []byte) error {
	if strings.TrimSpace(executablePath) == "" {
		return errors.New("executable path is required")
	}
	dir := filepath.Dir(executablePath)
	tmp, err := os.CreateTemp(dir, ".synocli-update-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp binary: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(binary); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp binary: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp binary: %w", err)
	}
	if err := os.Rename(tmpPath, executablePath); err != nil {
		return fmt.Errorf("replace executable: %w", err)
	}
	return nil
}
