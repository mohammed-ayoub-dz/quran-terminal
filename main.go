package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	appVersion = "v1.0.0"
	githubURL  = "https://github.com/quran-terminal/cli"
)
// later change
const titleArt = `
Quran Terminal Tool
`;


type Video struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Thumbnail   string `json:"thumbnail"`
}

var videoDB []Video
var dataFile = "sounds.json"
var favorites []int
var favoritesFile = "favorites.json"

func loadFavorites() error {
	file, err := os.Open(favoritesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	return decoder.Decode(&favorites)
}

func saveFavorites() error {
	file, err := os.Create(favoritesFile)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(favorites)
}

func isFavorite(id int) bool {
	for _, fid := range favorites {
		if fid == id {
			return true
		}
	}
	return false
}

func toggleFavorite(id int) error {
	idx := -1
	for i, fid := range favorites {
		if fid == id {
			idx = i
			break
		}
	}
	if idx >= 0 {
		favorites = append(favorites[:idx], favorites[idx+1:]...)
	} else {
		favorites = append(favorites, id)
	}
	return saveFavorites()
}

func loadVideos() error {
	file, err := os.Open(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&videoDB)
	if err != nil {
		return err
	}
	return nil
}

func saveVideos() error {
	file, err := os.Create(dataFile)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(videoDB)
	if err != nil {
		return err
	}
	return nil
}

func AddVideo(youtubeURL, title, description string) (Video, error) {
	if youtubeURL == "" {
		return Video{}, errors.New("URL cannot be empty")
	}
	if title == "" {
		return Video{}, errors.New("title cannot be empty")
	}
	maxID := 0
	for _, v := range videoDB {
		if v.ID > maxID {
			maxID = v.ID
		}
	}
	newID := maxID + 1
	newVideo := Video{
		ID:          newID,
		Title:       title,
		Description: description,
		URL:         youtubeURL,
		Thumbnail:   "",
	}
	videoDB = append(videoDB, newVideo)
	err := saveVideos()
	if err != nil {
		return Video{}, fmt.Errorf("failed to save: %w", err)
	}
	return newVideo, nil
}

func headerView(width int) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("5")).
		Align(lipgloss.Center).
		Width(width)
	title := titleStyle.Render(titleArt)
	versionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Align(lipgloss.Center).
		Width(width)
	versionLine := versionStyle.Render(appVersion)
	return lipgloss.JoinVertical(lipgloss.Center, title, versionLine)
}

func footerView(width int) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Align(lipgloss.Center).
		Width(width).
		Italic(true)
	return style.Render(githubURL)
}

func wrapWithHeaderFooter(width, height int, body string) string {
	header := headerView(width)
	footer := footerView(width)
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, header, body, footer),
	)
}

type mainMenuModel struct {
	cursor   int
	choices  []string
	selected string
	width    int
	height   int
}

func (m mainMenuModel) Init() tea.Cmd { return nil }

func (m mainMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter", " ":
			m.selected = m.choices[m.cursor]
			return m, tea.Quit
		case "q", "ctrl+c":
			m.selected = "Exit"
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m mainMenuModel) View() string {
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	normalStyle := lipgloss.NewStyle()
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2)

	var lines []string
	for i, choice := range m.choices {
		if m.cursor == i {
			lines = append(lines, selectedStyle.Render("> "+choice))
		} else {
			lines = append(lines, normalStyle.Render("  "+choice))
		}
	}

	boxContent := strings.Join(lines, "\n")
	boxed := boxStyle.Render(boxContent)

	body := lipgloss.Place(
		m.width, m.height-6,
		lipgloss.Center, lipgloss.Center,
		boxed,
	)
	return wrapWithHeaderFooter(m.width, m.height, body)
}

func runMainMenu() string {
	p := tea.NewProgram(
		mainMenuModel{choices: []string{"All Tracks", "Favorites", "Add video", "Exit"}},
		tea.WithAltScreen(),
	)
	finalModel, err := p.Run()
	if err != nil {
		fmt.Println("Error running main menu:", err)
		os.Exit(1)
	}
	m := finalModel.(mainMenuModel)
	return m.selected
}

type videoListModel struct {
	videos        []Video
	filtered      []Video
	cursor        int
	searchInput   textinput.Model
	viewport      viewport.Model
	selected      Video
	selectionMade bool
	quitting      bool
	width         int
	height        int
	favorites     map[int]bool
}

func initialVideoListModel(videos []Video) videoListModel {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 40

	vp := viewport.New(60, 15)

	favs := make(map[int]bool)
	for _, id := range favorites {
		favs[id] = true
	}

	return videoListModel{
		videos:      videos,
		filtered:    videos,
		searchInput: ti,
		viewport:    vp,
		favorites:   favs,
	}
}

func (m videoListModel) Init() tea.Cmd { return textinput.Blink }

func (m videoListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = m.width - 4
		m.viewport.Height = m.height - 12
		m.updateViewportContent()
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
				m.updateViewportContent()
			}
		case "down":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.updateViewportContent()
			}
		case "enter":
			if len(m.filtered) > 0 {
				m.selected = m.filtered[m.cursor]
				m.selectionMade = true
				return m, tea.Quit
			}
		case "f":
			if len(m.filtered) > 0 {
				id := m.filtered[m.cursor].ID
				toggleFavorite(id)
				m.favorites[id] = !m.favorites[id]
				m.updateViewportContent()
			}
		default:
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.filterVideos()
			if m.cursor >= len(m.filtered) {
				m.cursor = 0
			}
			m.updateViewportContent()
			return m, cmd
		}
	}
	return m, nil
}

func (m *videoListModel) updateViewportContent() {
	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	normalStyle := lipgloss.NewStyle()
	favStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

	var lines []string
	for i, v := range m.filtered {
		title := v.Title
		if len(title) > m.viewport.Width-6 {
			title = title[:m.viewport.Width-9] + "..."
		}
		cursorMark := " "
		if i == m.cursor {
			cursorMark = ">"
		}
		favMark := " "
		if m.favorites[v.ID] {
			favMark = favStyle.Render("*")
		}
		line := fmt.Sprintf("%s %s %s", cursorMark, favMark, title)
		if i == m.cursor {
			lines = append(lines, highlightStyle.Render(line))
		} else {
			lines = append(lines, normalStyle.Render(line))
		}
	}
	content := strings.Join(lines, "\n")
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
	if m.cursor >= 0 && m.cursor < len(m.filtered) {
		m.viewport.LineDown(m.cursor)
	}
}

func (m *videoListModel) filterVideos() {
	term := m.searchInput.Value()
	if term == "" {
		m.filtered = m.videos
		return
	}
	m.filtered = nil
	lowerTerm := strings.ToLower(term)
	for _, v := range m.videos {
		if strings.Contains(strings.ToLower(v.Title), lowerTerm) {
			m.filtered = append(m.filtered, v)
		}
	}
}

func (m videoListModel) View() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1)

	searchBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 1).
		Render(m.searchInput.View())

	legend := "F: toggle favorite"
	viewportBox := boxStyle.Render(m.viewport.View())
	content := lipgloss.JoinVertical(lipgloss.Left, searchBox, viewportBox, legend)

	body := lipgloss.Place(
		m.width, m.height-6,
		lipgloss.Center, lipgloss.Center,
		content,
	)
	return wrapWithHeaderFooter(m.width, m.height, body)
}

func runVideoList(videos []Video) (Video, bool) {
	if len(videos) == 0 {
		return Video{}, false
	}
	p := tea.NewProgram(
		initialVideoListModel(videos),
		tea.WithAltScreen(),
	)
	finalModel, err := p.Run()
	if err != nil {
		fmt.Println("Error running video list:", err)
		return Video{}, false
	}
	m := finalModel.(videoListModel)
	return m.selected, m.selectionMade
}

func mpvAvailable() bool {
	_, err := exec.LookPath("mpv")
	return err == nil
}

func installMpv() error {
	fmt.Println("Attempting to install mpv...")

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("apt"); err == nil {
			cmd = exec.Command("sudo", "apt", "install", "-y", "mpv")
		} else if _, err := exec.LookPath("pacman"); err == nil {
			cmd = exec.Command("sudo", "pacman", "-S", "--noconfirm", "mpv")
		} else if _, err := exec.LookPath("dnf"); err == nil {
			cmd = exec.Command("sudo", "dnf", "install", "-y", "mpv")
		} else if _, err := exec.LookPath("zypper"); err == nil {
			cmd = exec.Command("sudo", "zypper", "install", "-y", "mpv")
		} else {
			return errors.New("no supported package manager found (apt, pacman, dnf, zypper)")
		}
	case "windows":
		if _, err := exec.LookPath("winget"); err == nil {
			cmd = exec.Command("winget", "install", "--id", "mpv.net")
		} else if _, err := exec.LookPath("choco"); err == nil {
			cmd = exec.Command("choco", "install", "mpv", "-y")
		} else {
			return errors.New("no supported package manager found (winget or chocolatey)")
		}
	case "darwin":
		if _, err := exec.LookPath("brew"); err == nil {
			cmd = exec.Command("brew", "install", "mpv")
		} else {
			return errors.New("Homebrew not found; please install mpv manually")
		}
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}
	return nil
}

func mpvSendCommand(socketPath string, command map[string]interface{}) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	encoder := json.NewEncoder(conn)
	return encoder.Encode(command)
}

type playerModel struct {
	cmd        *exec.Cmd
	socketPath string
	videoTitle string
	playing    bool
	quitting   bool
	width      int
	height     int
	mpvError   string
}

func initialPlayerModel(video Video) (playerModel, error) {
	if !mpvAvailable() {
		return playerModel{}, errors.New("mpv not found in PATH")
	}
	socketPath := "/tmp/quran_mpv_socket"
	os.Remove(socketPath)
	cmd := exec.Command("mpv", "--no-video", "--input-ipc-server="+socketPath, video.URL)
	err := cmd.Start()
	if err != nil {
		return playerModel{}, fmt.Errorf("failed to start mpv: %w", err)
	}
	time.Sleep(500 * time.Millisecond)
	return playerModel{
		cmd:        cmd,
		socketPath: socketPath,
		videoTitle: video.Title,
		playing:    true,
	}, nil
}

func (m playerModel) Init() tea.Cmd { return nil }

func (m playerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case " ":
			if m.cmd != nil && m.cmd.Process != nil {
				m.playing = !m.playing
				mpvSendCommand(m.socketPath, map[string]interface{}{
					"command": []interface{}{"set_property", "pause", !m.playing},
				})
			}
		case "s":
			if m.cmd != nil && m.cmd.Process != nil {
				mpvSendCommand(m.socketPath, map[string]interface{}{
					"command": []interface{}{"stop"},
				})
				m.cmd.Process.Kill()
			}
			m.quitting = true
			return m, tea.Quit
		case "q", "ctrl+c":
			if m.cmd != nil && m.cmd.Process != nil {
				mpvSendCommand(m.socketPath, map[string]interface{}{
					"command": []interface{}{"quit"},
				})
				m.cmd.Process.Kill()
			}
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m playerModel) View() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2)

	var bodyContent string
	if m.mpvError != "" {
		bodyContent = fmt.Sprintf("Error: %s\n\nPress any key to return.", m.mpvError)
	} else {
		status := "Playing"
		if !m.playing {
			status = "Paused"
		}
		bodyContent = fmt.Sprintf("Now: %s\n[ %s ]\n\nSpace: Play/Pause  S: Stop  Q: Back",
			m.videoTitle, status)
	}

	boxed := boxStyle.Render(bodyContent)

	body := lipgloss.Place(
		m.width, m.height-6,
		lipgloss.Center, lipgloss.Center,
		boxed,
	)
	return wrapWithHeaderFooter(m.width, m.height, body)
}

func runPlayer(video Video) {
	if !mpvAvailable() {
		err := installMpv()
		if err != nil {
			model := playerModel{
				mpvError: fmt.Sprintf("mpv installation failed: %v", err),
			}
			p := tea.NewProgram(model, tea.WithAltScreen())
			p.Run()
			return
		}
		if !mpvAvailable() {
			model := playerModel{
				mpvError: "mpv installed but still not found in PATH. Restart may be required.",
			}
			p := tea.NewProgram(model, tea.WithAltScreen())
			p.Run()
			return
		}
	}

	model, err := initialPlayerModel(video)
	if err != nil {
		model = playerModel{
			mpvError: err.Error(),
		}
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running player:", err)
	}

	if model.cmd != nil && model.cmd.Process != nil && !model.quitting {
		model.cmd.Process.Kill()
	}
}

type addVideoModel struct {
	urlInput   textinput.Model
	titleInput textinput.Model
	descInput  textinput.Model
	focusIndex int
	submitted  bool
	err        string
	width      int
	height     int
}

func initialAddVideoModel() addVideoModel {
	url := textinput.New()
	url.Placeholder = "YouTube URL"
	url.Focus()
	url.CharLimit = 200
	url.Width = 40

	title := textinput.New()
	title.Placeholder = "Title"
	title.CharLimit = 100
	title.Width = 40

	desc := textinput.New()
	desc.Placeholder = "Description (optional)"
	desc.CharLimit = 200
	desc.Width = 40

	return addVideoModel{
		urlInput:   url,
		titleInput: title,
		descInput:  desc,
		focusIndex: 0,
	}
}

func (m addVideoModel) Init() tea.Cmd { return textinput.Blink }

func (m addVideoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			m.focusIndex = (m.focusIndex + 1) % 3
			m.updateFocus()
		case "up":
			m.focusIndex = (m.focusIndex - 1 + 3) % 3
			m.updateFocus()
		case "enter":
			if m.focusIndex == 2 {
				m.submit()
				m.submitted = true
				return m, tea.Quit
			}
			m.focusIndex = (m.focusIndex + 1) % 3
			m.updateFocus()
		case "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	switch m.focusIndex {
	case 0:
		m.urlInput, cmd = m.urlInput.Update(msg)
	case 1:
		m.titleInput, cmd = m.titleInput.Update(msg)
	case 2:
		m.descInput, cmd = m.descInput.Update(msg)
	}
	return m, cmd
}

func (m *addVideoModel) updateFocus() {
	m.urlInput.Blur()
	m.titleInput.Blur()
	m.descInput.Blur()
	switch m.focusIndex {
	case 0:
		m.urlInput.Focus()
	case 1:
		m.titleInput.Focus()
	case 2:
		m.descInput.Focus()
	}
}

func (m *addVideoModel) submit() {
	url := m.urlInput.Value()
	title := m.titleInput.Value()
	desc := m.descInput.Value()
	_, err := AddVideo(url, title, desc)
	if err != nil {
		m.err = err.Error()
		return
	}
	m.err = ""
}

func (m addVideoModel) View() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2)

	inputs := lipgloss.JoinVertical(lipgloss.Left,
		"URL:   "+m.urlInput.View(),
		"Title: "+m.titleInput.View(),
		"Desc:  "+m.descInput.View(),
		"",
		"Enter on Desc to submit, Esc to cancel.",
	)

	if m.err != "" {
		inputs += "\nError: " + m.err
	}

	boxed := boxStyle.Render(inputs)

	body := lipgloss.Place(
		m.width, m.height-6,
		lipgloss.Center, lipgloss.Center,
		boxed,
	)
	return wrapWithHeaderFooter(m.width, m.height, body)
}

func runAddVideo() {
	p := tea.NewProgram(
		initialAddVideoModel(),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running add video:", err)
	}
}

func main() {
	err := loadVideos()
	if err != nil {
		fmt.Printf("Warning: could not load videos: %v\n", err)
		os.Exit(1)
	}
	err = loadFavorites()
	if err != nil {
		fmt.Printf("Warning: could not load favorites: %v\n", err)
	}

	for {
		choice := runMainMenu()
		switch choice {
		case "All Tracks":
			video, ok := runVideoList(videoDB)
			if ok {
				runPlayer(video)
			}
		case "Favorites":
			favVideos := []Video{}
			for _, v := range videoDB {
				if isFavorite(v.ID) {
					favVideos = append(favVideos, v)
				}
			}
			video, ok := runVideoList(favVideos)
			if ok {
				runPlayer(video)
			}
		case "Add video":
			runAddVideo()
		case "Exit":
			return
		default:
			return
		}
	}
}