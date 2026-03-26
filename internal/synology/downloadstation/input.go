package downloadstation

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// AddInputType identifies the kind of input for adding a download task.
type AddInputType string

const (
	AddInputURL     AddInputType = "url"
	AddInputMagnet  AddInputType = "magnet"
	AddInputTorrent AddInputType = "torrent"
)

// DetectAddInputKind identifies whether input is a magnet URI, local torrent file, or URL.
func DetectAddInputKind(input string) (AddInputType, error) {
	lower := strings.ToLower(input)
	if strings.HasPrefix(lower, "magnet:") {
		return AddInputMagnet, nil
	}
	st, err := os.Stat(input)
	if err == nil {
		if st.IsDir() {
			return "", fmt.Errorf("input %q is a directory, expected a torrent file", input)
		}
		return AddInputTorrent, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat input: %w", err)
	}
	u, err := url.Parse(input)
	if err == nil && u.Scheme != "" && !strings.EqualFold(u.Scheme, "magnet") {
		return AddInputURL, nil
	}
	return "", fmt.Errorf("cannot detect input type; expected magnet URI, existing torrent file path, or URL with scheme")
}
