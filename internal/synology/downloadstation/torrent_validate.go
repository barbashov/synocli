package downloadstation

import (
	"fmt"
	"os"
	"strconv"
)

func ValidateTorrentFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read torrent file: %w", err)
	}
	p := torrentBencodeParser{data: data}
	root, err := p.parseValue()
	if err != nil {
		return fmt.Errorf("invalid bencode: %w", err)
	}
	if p.pos != len(data) {
		return fmt.Errorf("invalid bencode: trailing data")
	}
	rootMap, ok := root.(map[string]any)
	if !ok {
		return fmt.Errorf("top-level bencode value must be dictionary")
	}
	info, ok := rootMap["info"]
	if !ok {
		return fmt.Errorf("missing info dictionary")
	}
	if _, ok := info.(map[string]any); !ok {
		return fmt.Errorf("info value must be dictionary")
	}
	return nil
}

type torrentBencodeParser struct {
	data []byte
	pos  int
}

func (p *torrentBencodeParser) parseValue() (any, error) {
	if p.pos >= len(p.data) {
		return nil, fmt.Errorf("unexpected end of data")
	}
	switch p.data[p.pos] {
	case 'd':
		return p.parseDict()
	case 'l':
		return p.parseList()
	case 'i':
		return p.parseInt()
	default:
		if p.data[p.pos] < '0' || p.data[p.pos] > '9' {
			return nil, fmt.Errorf("invalid token %q", p.data[p.pos])
		}
		return p.parseString()
	}
}

func (p *torrentBencodeParser) parseDict() (map[string]any, error) {
	p.pos++
	out := map[string]any{}
	for {
		if p.pos >= len(p.data) {
			return nil, fmt.Errorf("unterminated dictionary")
		}
		if p.data[p.pos] == 'e' {
			p.pos++
			return out, nil
		}
		k, err := p.parseString()
		if err != nil {
			return nil, fmt.Errorf("decode dictionary key: %w", err)
		}
		key, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("invalid dictionary key")
		}
		v, err := p.parseValue()
		if err != nil {
			return nil, fmt.Errorf("decode dictionary value for key %q: %w", key, err)
		}
		out[key] = v
	}
}

func (p *torrentBencodeParser) parseList() ([]any, error) {
	p.pos++
	out := []any{}
	for {
		if p.pos >= len(p.data) {
			return nil, fmt.Errorf("unterminated list")
		}
		if p.data[p.pos] == 'e' {
			p.pos++
			return out, nil
		}
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
}

func (p *torrentBencodeParser) parseInt() (int64, error) {
	p.pos++
	if p.pos >= len(p.data) {
		return 0, fmt.Errorf("unterminated integer")
	}
	start := p.pos
	for p.pos < len(p.data) && p.data[p.pos] != 'e' {
		p.pos++
	}
	if p.pos >= len(p.data) {
		return 0, fmt.Errorf("unterminated integer")
	}
	if p.pos == start {
		return 0, fmt.Errorf("empty integer")
	}
	raw := string(p.data[start:p.pos])
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q", raw)
	}
	p.pos++
	return n, nil
}

func (p *torrentBencodeParser) parseString() (any, error) {
	start := p.pos
	for p.pos < len(p.data) && p.data[p.pos] >= '0' && p.data[p.pos] <= '9' {
		p.pos++
	}
	if p.pos == start || p.pos >= len(p.data) || p.data[p.pos] != ':' {
		return nil, fmt.Errorf("invalid string length")
	}
	lengthRaw := string(p.data[start:p.pos])
	length, err := strconv.Atoi(lengthRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid string length %q", lengthRaw)
	}
	p.pos++
	end := p.pos + length
	if length < 0 || end > len(p.data) {
		return nil, fmt.Errorf("string exceeds input")
	}
	s := string(p.data[p.pos:end])
	p.pos = end
	return s, nil
}
