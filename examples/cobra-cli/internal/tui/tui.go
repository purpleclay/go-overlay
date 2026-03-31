package tui

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"github.com/purpleclay/go-overlay/examples/cobra-cli/internal/suggest"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Colour palette mirrors catto's aesthetic.
var (
	crimson = lipgloss.Color("#e94560")
	muted   = lipgloss.Color("#8888aa")
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(crimson).
			Bold(true)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(muted)

	inputContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(crimson).
				Padding(0, 1)

	breedStyle = lipgloss.NewStyle().
			Foreground(crimson).
			Bold(true)

	reasonStyle = lipgloss.NewStyle().
			Foreground(muted).
			Italic(true)

	hintStyle = lipgloss.NewStyle().
			Foreground(muted)
)

type tuiState int

const (
	stateInput tuiState = iota
	stateResult
)

type imageLoadedMsg struct {
	img image.Image
}

// Model is the bubbletea model for the doggo TUI.
type Model struct {
	state  tuiState
	input  textinput.Model
	breed  suggest.Breed
	img    image.Image
	width  int
	height int
}

// New returns an initialised Model ready to be passed to tea.NewProgram.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "enter your name"
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 54

	return Model{
		state: stateInput,
		input: ti,
	}
}

func loadImageCmd(filename string) tea.Cmd {
	return func() tea.Msg {
		f, err := staticFiles.Open("static/" + filename)
		if err != nil {
			return imageLoadedMsg{}
		}
		defer f.Close()

		img, _, err := image.Decode(f)
		if err != nil {
			return imageLoadedMsg{}
		}
		return imageLoadedMsg{img: img}
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case imageLoadedMsg:
		m.img = msg.img
		m.state = stateResult
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.state == stateResult {
				m.state = stateInput
				m.input.SetValue("")
				m.input.Focus()
				m.img = nil
				return m, textinput.Blink
			}
			return m, tea.Quit
		case "q":
			if m.state == stateResult {
				return m, tea.Quit
			}
		case "enter":
			if m.state == stateInput {
				name := strings.TrimSpace(m.input.Value())
				if name == "" {
					return m, nil
				}
				m.breed = suggest.ForName(name)
				return m, loadImageCmd(m.breed.Image)
			}
		}
	}

	if m.state == stateInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	switch m.state {
	case stateInput:
		return m.viewInput()
	case stateResult:
		return m.viewResult()
	}
	return ""
}

func (m Model) viewInput() string {
	return fmt.Sprintf(
		"\n%s\n%s\n\n%s\n%s",
		titleStyle.Render("doggo"),
		subtitleStyle.Render("find out what dog you deserve"),
		inputContainerStyle.Render(m.input.View()),
		hintStyle.Render("  press ↵ to reveal your dog"),
	)
}

func (m Model) viewResult() string {
	// Scale the image to fit the terminal width, up to a comfortable maximum.
	imgCols := 60
	imgRows := 24
	if m.width > 0 && m.width-4 < imgCols {
		imgCols = m.width - 4
		imgRows = imgCols / 2
	}

	imgStr := ""
	if m.img != nil {
		imgStr = renderImage(m.img, imgCols, imgRows)
	}

	return fmt.Sprintf(
		"\n%s\n\n%s\n\n%s\n%s\n\n%s",
		titleStyle.Render("doggo"),
		imgStr,
		breedStyle.Render(strings.ToUpper(m.breed.Name)),
		reasonStyle.Render(m.breed.Reason),
		hintStyle.Render("  press q to quit · esc to go back"),
	)
}

// renderImage converts src to a string of ▄ half-block characters with 24-bit
// ANSI colour codes. Each terminal row encodes two image rows: the top pixel
// becomes the cell background and the bottom pixel becomes the foreground (▄).
// Nearest-neighbor scaling preserves the blocky, pixelated look.
func renderImage(src image.Image, cols, rows int) string {
	bounds := src.Bounds()
	sw, sh := bounds.Dx(), bounds.Dy()
	totalRows := rows * 2

	var out strings.Builder
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			sx := col * sw / cols
			syTop := (row * 2) * sh / totalRows
			syBot := (row*2 + 1) * sh / totalRows

			tr, tg, tb, _ := src.At(bounds.Min.X+sx, bounds.Min.Y+syTop).RGBA()
			br, bg, bb, _ := src.At(bounds.Min.X+sx, bounds.Min.Y+syBot).RGBA()

			// ▄ foreground = bottom pixel, background = top pixel.
			out.WriteString(fmt.Sprintf(
				"\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm▄\033[0m",
				br>>8, bg>>8, bb>>8,
				tr>>8, tg>>8, tb>>8,
			))
		}
		if row < rows-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}
