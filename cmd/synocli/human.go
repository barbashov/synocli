package main

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

const ansiClearScreen = "\x1b[H\x1b[2J"

type kvField struct {
	Label string
	Value string
}

type humanUI struct {
	renderer *lipgloss.Renderer
	styled   bool
	tty      bool
}

func newHumanUI(w io.Writer) humanUI {
	tty := isTTYWriter(w)
	renderer := lipgloss.NewRenderer(w, termenv.WithTTY(tty))
	noColor := termenv.EnvNoColor()
	styled := tty && !noColor
	if !styled {
		renderer.SetColorProfile(termenv.Ascii)
	}
	return humanUI{renderer: renderer, styled: styled, tty: tty}
}

func isTTYWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func (u humanUI) style() lipgloss.Style {
	return u.renderer.NewStyle()
}

func (u humanUI) title(text string) string {
	s := u.style().Bold(true)
	if u.styled {
		s = s.Foreground(lipgloss.AdaptiveColor{Light: "27", Dark: "117"})
	}
	return s.Render(text)
}

func (u humanUI) muted(text string) string {
	s := u.style()
	if u.styled {
		s = s.Foreground(lipgloss.AdaptiveColor{Light: "244", Dark: "245"})
	}
	return s.Render(text)
}

func (u humanUI) status(text, normalized string) string {
	s := u.style()
	if u.styled {
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

func (u humanUI) badge(kind string) string {
	upper := strings.ToUpper(kind)
	if !u.styled {
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

func printError(w io.Writer, err error) {
	ui := newHumanUI(w)
	_, _ = fmt.Fprintf(w, "%s %s\n", ui.badge("error"), err.Error())
}

func printKVBlock(w io.Writer, title string, fields []kvField) {
	ui := newHumanUI(w)
	if title != "" {
		_, _ = fmt.Fprintln(w, ui.title(title))
	}
	maxLabel := 0
	for _, f := range fields {
		if len(f.Label) > maxLabel {
			maxLabel = len(f.Label)
		}
	}
	label := ui.style().Bold(true).Width(maxLabel + 1)
	if ui.styled {
		label = label.Foreground(lipgloss.AdaptiveColor{Light: "63", Dark: "111"})
	}
	for _, f := range fields {
		_, _ = fmt.Fprintln(w, label.Render(f.Label+":")+" "+f.Value)
	}
}

func printTable(w io.Writer, headers []string, rows [][]string) {
	ui := newHumanUI(w)
	t := table.New().
		Headers(headers...).
		Rows(rows...).
		BorderRow(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				s := ui.style().Bold(true)
				if ui.styled {
					s = s.Foreground(lipgloss.AdaptiveColor{Light: "63", Dark: "111"})
				}
				return s
			}
			return ui.style()
		})
	if ui.styled {
		t = t.BorderStyle(ui.style().Foreground(lipgloss.AdaptiveColor{Light: "249", Dark: "238"}))
	}
	_, _ = fmt.Fprintln(w, t.Render())
}

func printWatchHeader(w io.Writer, snapTime time.Time, taskCount int, ids, statuses []string) {
	idFilter := "-"
	if len(ids) > 0 {
		idFilter = strings.Join(ids, ",")
	}
	statusFilter := "-"
	if len(statuses) > 0 {
		statusFilter = strings.Join(statuses, ",")
	}
	printKVBlock(w, "Download Station Watch", []kvField{
		{Label: "Timestamp", Value: snapTime.Format(time.RFC3339)},
		{Label: "Tasks", Value: fmt.Sprintf("%d", taskCount)},
		{Label: "ID Filter", Value: idFilter},
		{Label: "Status Filter", Value: statusFilter},
	})
}
