package cmdutil

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

const AnsiClearScreen = "\x1b[H\x1b[2J"

type KVField struct {
	Label string
	Value string
}

type HumanUI struct {
	renderer *lipgloss.Renderer
	Styled   bool
	Tty      bool
}

func NewHumanUI(w io.Writer) HumanUI {
	tty := isTTYWriter(w)
	renderer := lipgloss.NewRenderer(w, termenv.WithTTY(tty))
	noColor := termenv.EnvNoColor()
	styled := tty && !noColor
	if !styled {
		renderer.SetColorProfile(termenv.Ascii)
	}
	return HumanUI{renderer: renderer, Styled: styled, Tty: tty}
}

func isTTYWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func (u HumanUI) style() lipgloss.Style {
	return u.renderer.NewStyle()
}

func (u HumanUI) Title(text string) string {
	s := u.style().Bold(true)
	if u.Styled {
		s = s.Foreground(lipgloss.AdaptiveColor{Light: "27", Dark: "117"})
	}
	return s.Render(text)
}

func (u HumanUI) Muted(text string) string {
	s := u.style()
	if u.Styled {
		s = s.Foreground(lipgloss.AdaptiveColor{Light: "244", Dark: "245"})
	}
	return s.Render(text)
}

func (u HumanUI) Status(text, normalized string) string {
	s := u.style()
	if u.Styled {
		switch normalized {
		case "finished", "seeding":
			s = s.Foreground(lipgloss.Color("42"))
		case "downloading", "finishing":
			s = s.Foreground(lipgloss.Color("39"))
		case "paused", "waiting":
			s = s.Foreground(lipgloss.Color("214"))
		case "error":
			s = s.Foreground(lipgloss.Color("196"))
		default:
			s = s.Foreground(lipgloss.AdaptiveColor{Light: "241", Dark: "248"})
		}
	}
	return s.Render(text)
}

func (u HumanUI) Badge(kind string) string {
	upper := strings.ToUpper(kind)
	if !u.Styled {
		return upper
	}
	bg := lipgloss.Color("240")
	switch kind {
	case "ok":
		bg = lipgloss.Color("35")
	case "warn":
		bg = lipgloss.Color("214")
	case "error":
		bg = lipgloss.Color("196")
	}
	return u.style().
		Bold(true).
		Foreground(lipgloss.Color("230")).
		Background(bg).
		Padding(0, 1).
		Render(upper)
}

func PrintError(w io.Writer, err error) {
	ui := NewHumanUI(w)
	_, _ = fmt.Fprintf(w, "%s %s\n", ui.Badge("error"), err.Error())
}

func PrintKVBlock(w io.Writer, title string, fields []KVField) {
	ui := NewHumanUI(w)
	if title != "" {
		_, _ = fmt.Fprintln(w, ui.Title(title))
	}
	maxLabel := 0
	for _, f := range fields {
		if len(f.Label) > maxLabel {
			maxLabel = len(f.Label)
		}
	}
	label := ui.style().Bold(true).Width(maxLabel + 1)
	if ui.Styled {
		label = label.Foreground(lipgloss.AdaptiveColor{Light: "63", Dark: "111"})
	}
	for _, f := range fields {
		_, _ = fmt.Fprintln(w, label.Render(f.Label+":")+" "+f.Value)
	}
}

func PrintTable(w io.Writer, headers []string, rows [][]string) {
	ui := NewHumanUI(w)
	t := table.New().
		Headers(headers...).
		Rows(rows...).
		BorderRow(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				s := ui.style().Bold(true)
				if ui.Styled {
					s = s.Foreground(lipgloss.AdaptiveColor{Light: "63", Dark: "111"})
				}
				return s
			}
			return ui.style()
		})
	if ui.Styled {
		t = t.BorderStyle(ui.style().Foreground(lipgloss.AdaptiveColor{Light: "249", Dark: "238"}))
	}
	_, _ = fmt.Fprintln(w, t.Render())
}

func PrintWatchHeader(w io.Writer, snapTime time.Time, taskCount int, ids, statuses []string) {
	idFilter := "-"
	if len(ids) > 0 {
		idFilter = strings.Join(ids, ",")
	}
	statusFilter := "-"
	if len(statuses) > 0 {
		statusFilter = strings.Join(statuses, ",")
	}
	PrintKVBlock(w, "Download Station Watch", []KVField{
		{Label: "Timestamp", Value: snapTime.Format(time.RFC3339)},
		{Label: "Tasks", Value: fmt.Sprintf("%d", taskCount)},
		{Label: "ID Filter", Value: idFilter},
		{Label: "Status Filter", Value: statusFilter},
	})
}
