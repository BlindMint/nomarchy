package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Step constants
const (
	stepKeyboard = iota
	stepUsername
	stepPassword
	stepHostname
	stepWifi
	stepTimezone
	stepDisk
	stepConfirm
)

var (
	// Brand colors
	brandPurple    = lipgloss.Color("#845DF9")
	brandPurpleDim = lipgloss.Color("#6B4DC7")
	bgDark         = lipgloss.Color("#1E1E2E")
	bgCard         = lipgloss.Color("#252535")
	bgInput        = lipgloss.Color("#313244")
	textPrimary    = lipgloss.Color("#CDD6F4")
	textSecondary  = lipgloss.Color("#A6ADC8")
	textMuted      = lipgloss.Color("#6C7086")
	textSuccess    = lipgloss.Color("#A6E3A1")
	textError      = lipgloss.Color("#F38BA8")
	textWarning    = lipgloss.Color("#F9E2AF")
	borderColor    = lipgloss.Color("#45475A")
	borderActive   = lipgloss.Color("#845DF9")

	// Styles
	headerStyle = lipgloss.NewStyle().
			Foreground(textPrimary).
			Background(bgDark).
			Padding(1, 2).
			Width(70)

	titleStyle = lipgloss.NewStyle().
			Foreground(brandPurple).
			Bold(true).
			MarginBottom(1)

	cardStyle = lipgloss.NewStyle().
			Background(bgCard).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1, 3).
			Height(boxHeight).
			Width(66)

	cardActiveStyle = lipgloss.NewStyle().
			Background(bgCard).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderActive).
			Padding(1, 3).
			Height(boxHeight).
			Width(66)

	inputStyle = lipgloss.NewStyle().
			Background(bgInput).
			Foreground(textPrimary).
			Padding(0, 1).
			Width(60)

	inputActiveStyle = lipgloss.NewStyle().
				Background(bgInput).
				Foreground(textPrimary).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(brandPurple).
				Padding(0, 1).
				Width(60)

	labelStyle = lipgloss.NewStyle().
			Foreground(textSecondary).
			MarginBottom(0)

	helpStyle = lipgloss.NewStyle().
			Foreground(textMuted).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(textError)

	successStyle = lipgloss.NewStyle().
			Foreground(textSuccess)

	warningStyle = lipgloss.NewStyle().
			Foreground(textWarning).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(bgDark).
			Background(brandPurple).
			Bold(true).
			Padding(0, 1)

	stepActiveStyle = lipgloss.NewStyle().
			Foreground(brandPurple).
			Bold(true)

	stepDoneStyle = lipgloss.NewStyle().
			Foreground(textSuccess)

	stepTodoStyle = lipgloss.NewStyle().
			Foreground(textMuted)

	editBannerStyle = lipgloss.NewStyle().
			Foreground(bgDark).
			Background(textWarning).
			Bold(true).
			Padding(0, 2).
			Width(66).
			Align(lipgloss.Center)

	mutedStyle = lipgloss.NewStyle().
			Foreground(textMuted)

	boxHeight         = 14
	boxLineTitle      = 3
	boxLineHelp       = 14
	boxLineContentEnd = 12
	boxLineDesc       = 13
)

type model struct {
	step         int
	substep      int
	editMode     bool
	editReturnTo int

	// Input data
	keyboards      []keyboardOption
	selectedKbd    int
	keyboardFilter string
	username       string
	password       string
	confirmPass    string
	luksPass       string
	confirmLuks    string
	hostname       string
	timezone       string
	ssids          []string
	selectedSsid   int
	manualWifi     bool
	manualSsid     string
	wifiPass       string
	message        string
	timezones      []string
	filteredTzs    []string
	selectedTz     int
	tzFilter       string
	disks          []diskOption
	selectedDisk   int
	disk           string

	// State
	err          string
	confirmStep  bool
	complete     bool
	width        int
	height       int
	scanning     bool
	spinnerFrame int
}

type keyboardOption struct {
	name   string
	layout string
}

type diskOption struct {
	name   string
	info   string
	device string
}

type tickMsg time.Time

type wifiResultMsg struct {
	success bool
}

type wifiScanResultMsg struct {
	ssids []string
}

type spinnerTickMsg struct{}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}

func newModel() model {
	keyboards := getKeyboards()
	timezones := getTimezones()
	ssids := append([]string{"Manual entry"}, getSSIDs()...)
	return model{
		step:         stepKeyboard,
		substep:      0,
		keyboards:    keyboards,
		selectedKbd:  9, // Default to English (US)
		hostname:     "archlinux",
		timezones:    timezones,
		filteredTzs:  timezones,
		selectedTz:   0,
		ssids:        ssids,
		selectedSsid: 0,
		manualWifi:   false,
		disks:        getDisks(),
		selectedDisk: 0,
		editMode:     false,
		editReturnTo: stepConfirm,
	}
}

func getKeyboards() []keyboardOption {
	return []keyboardOption{
		{"Azerbaijani", "azerty"},
		{"Belgian", "be-latin1"},
		{"Bosnian", "ba"},
		{"Bulgarian", "bg-cp1251"},
		{"Croatian", "croat"},
		{"Czech", "cz"},
		{"Danish", "dk-latin1"},
		{"Dutch", "nl"},
		{"English (UK)", "uk"},
		{"English (US)", "us"},
		{"English (US, Dvorak)", "dvorak"},
		{"Estonian", "et"},
		{"Finnish", "fi"},
		{"French", "fr"},
		{"French (Canada)", "cf"},
		{"French (Switzerland)", "fr_CH"},
		{"Georgian", "ge"},
		{"German", "de"},
		{"German (Switzerland)", "de_CH-latin1"},
		{"Greek", "gr"},
		{"Hebrew", "il"},
		{"Hungarian", "hu"},
		{"Icelandic", "is-latin1"},
		{"Irish", "ie"},
		{"Italian", "it"},
		{"Japanese", "jp106"},
		{"Kazakh", "kazakh"},
		{"Korean", "kr"},
		{"Latvian", "lv"},
		{"Lithuanian", "lt"},
		{"Macedonian", "mk-utf"},
		{"Norwegian", "no-latin1"},
		{"Polish", "pl"},
		{"Portuguese", "pt-latin1"},
		{"Portuguese (Brazil)", "br-abnt2"},
		{"Romanian", "ro"},
		{"Russian", "ru"},
		{"Serbian", "sr-latin"},
		{"Slovak", "sk-qwertz"},
		{"Slovenian", "slovene"},
		{"Spanish", "es"},
		{"Spanish (Latin American)", "la-latin1"},
		{"Swedish", "sv-latin1"},
		{"Turkish", "trq"},
		{"Ukrainian", "ua"},
	}
}

func getTimezones() []string {
	cmd := exec.Command("timedatectl", "list-timezones")
	out, err := cmd.Output()
	if err != nil {
		// Fallback common timezones if command fails
		return []string{
			"UTC",
			"America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles",
			"America/Toronto", "America/Vancouver", "America/Mexico_City",
			"Europe/London", "Europe/Paris", "Europe/Berlin", "Europe/Madrid",
			"Europe/Rome", "Europe/Amsterdam", "Europe/Stockholm", "Europe/Oslo",
			"Asia/Tokyo", "Asia/Shanghai", "Asia/Hong_Kong", "Asia/Singapore",
			"Asia/Seoul", "Asia/Mumbai", "Asia/Dubai", "Asia/Bangkok",
			"Australia/Sydney", "Australia/Melbourne", "Pacific/Auckland",
		}
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n")
}

func getDisks() []diskOption {
	cmd := exec.Command("lsblk", "-dpno", "NAME,TYPE,SIZE,MODEL,VENDOR")
	out, err := cmd.Output()
	if err != nil {
		return []diskOption{}
	}

	var disks []diskOption
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[1] != "disk" {
			continue
		}

		device := fields[0]
		size := ""
		model := ""
		if len(fields) > 2 {
			size = fields[2]
		}
		if len(fields) > 4 {
			model = strings.Join(fields[4:], " ")
		}

		info := device
		if size != "" {
			info = fmt.Sprintf("%s (%s)", device, size)
		}
		if model != "" {
			info = fmt.Sprintf("%s - %s", info, model)
		}

		disks = append(disks, diskOption{
			name:   info,
			device: device,
		})
	}
	return disks
}

func getWifiDevice() string {
	cmd := exec.Command("iwctl", "device", "list")
	out, err := cmd.Output()
	if err != nil {
		return "wlan0"
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines[2:] { // skip header
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[3] == "wifi" {
			return fields[0]
		}
	}
	return "wlan0"
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripAnsi(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

func parseSSIDs(out []byte) []string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var ssids []string
	for _, line := range lines[4:] { // skip header
		line = strings.TrimSpace(stripAnsi(line))
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		ssid := fields[0]
		if ssid == ">" && len(fields) > 1 {
			ssid = fields[1]
		}
		if ssid != "" {
			ssids = append(ssids, ssid)
		}
	}
	return ssids
}

// getSSIDs reads iwd's cached scan results without triggering a new scan.
func getSSIDs() []string {
	device := getWifiDevice()
	out, err := exec.Command("iwctl", "station", device, "get-networks").Output()
	if err != nil {
		return []string{}
	}
	return parseSSIDs(out)
}

// scanAndGetSSIDs triggers a fresh scan (used on manual refresh with 'r').
func scanAndGetSSIDs() []string {
	device := getWifiDevice()
	exec.Command("iwctl", "station", device, "scan").Run()
	time.Sleep(4 * time.Second)
	out, err := exec.Command("iwctl", "station", device, "get-networks").Output()
	if err != nil {
		return []string{}
	}
	return parseSSIDs(out)
}

func connectWifi(device, ssid, passphrase string) tea.Cmd {
	return func() tea.Msg {
		if passphrase != "" {
			exec.Command("iwctl", "station", device, "connect", ssid, "--passphrase", passphrase).Run()
		} else {
			exec.Command("iwctl", "station", device, "connect", ssid).Run()
		}
		time.Sleep(5 * time.Second)
		out, err := exec.Command("iwctl", "station", device, "show").Output()
		if err != nil {
			return wifiResultMsg{success: false}
		}
		for _, line := range strings.Split(stripAnsi(string(out)), "\n") {
			fields := strings.Fields(line)
			// Look for "State connected" (not "disconnected" or "disconnecting")
			if len(fields) == 2 && fields[0] == "State" && fields[1] == "connected" {
				return wifiResultMsg{success: true}
			}
		}
		return wifiResultMsg{success: false}
	}
}

func (m model) Init() tea.Cmd {
	return tea.WindowSize()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tickMsg:
		m = m.advanceStep()
	case wifiResultMsg:
		if msg.success {
			m.message = "Connection successful!"
		} else {
			m.message = "Connection failed, but installation will continue."
		}
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
	case wifiScanResultMsg:
		m.ssids = append([]string{"Manual entry"}, msg.ssids...)
		m.selectedSsid = 0
		m.scanning = false
	case spinnerTickMsg:
		if m.scanning {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
			return m, spinnerTick()
		}
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global quit
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	switch m.step {
	case stepKeyboard:
		return m.handleKeyboardStep(msg)
	case stepUsername:
		return m.handleUsernameStep(msg)
	case stepPassword:
		switch m.substep {
		case 0:
			return m.handlePasswordStep(msg)
		case 1:
			return m.handleConfirmPasswordStep(msg)
		case 2:
			return m.handleLuksPassStep(msg)
		case 3:
			return m.handleConfirmLuksStep(msg)
		}
	case stepHostname:
		return m.handleHostnameStep(msg)
	case stepWifi:
		if m.substep == 0 {
			return m.handleWifiSsidStep(msg)
		} else if m.substep == 1 && m.manualWifi {
			return m.handleWifiManualSsidStep(msg)
		} else if m.substep == 3 {
			// Connecting, ignore input
			return m, nil
		}
		return m.handleWifiPassStep(msg)
	case stepTimezone:
		return m.handleTimezoneStep(msg)
	case stepDisk:
		return m.handleDiskStep(msg)
	case stepConfirm:
		return m.handleConfirmStep(msg)
	}
	return m, nil
}

func (m model) advanceStep() model {
	if m.editMode {
		m.step = m.editReturnTo
		m.editMode = false
	} else {
		m.step++
	}
	m.substep = 0
	m.err = ""
	return m
}

func (m model) goBack() model {
	if m.editMode {
		m.step = m.editReturnTo
		m.editMode = false
	} else if m.step > 0 {
		m.step--
	}
	m.substep = 0
	m.err = ""
	return m
}

func (m model) handleKeyboardStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.selectedKbd > 0 {
			m.selectedKbd--
		}
	case tea.KeyDown:
		if m.selectedKbd < len(m.keyboards)-1 {
			m.selectedKbd++
		}
	case tea.KeyEnter:
		m = m.advanceStep()
	case tea.KeyEsc:
		if m.editMode {
			m = m.goBack()
		}
	default:
		// Filter typing
		if len(msg.Runes) > 0 && unicode.IsLetter(msg.Runes[0]) {
			m.keyboardFilter += strings.ToLower(string(msg.Runes))
			// Find first match
			for i, kbd := range m.keyboards {
				if strings.Contains(strings.ToLower(kbd.name), m.keyboardFilter) {
					m.selectedKbd = i
					break
				}
			}
		} else if msg.Type == tea.KeyBackspace && len(m.keyboardFilter) > 0 {
			m.keyboardFilter = m.keyboardFilter[:len(m.keyboardFilter)-1]
		}
	}
	return m, nil
}

func (m model) handleWifiSsidStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.selectedSsid > 0 {
			m.selectedSsid--
		}
	case tea.KeyDown:
		if m.selectedSsid < len(m.ssids)-1 {
			m.selectedSsid++
		}
	case tea.KeyEnter:
		if len(m.ssids) > 0 {
			if m.selectedSsid == 0 {
				m.manualWifi = true
				m.manualSsid = ""
			} else {
				m.manualWifi = false
			}
			m.substep = 1
		}
	case tea.KeyRunes:
		if len(msg.Runes) == 1 && (msg.Runes[0] == 'r' || msg.Runes[0] == 'R') {
			m.scanning = true
			return m, tea.Batch(
				func() tea.Msg { return wifiScanResultMsg{ssids: scanAndGetSSIDs()} },
				spinnerTick(),
			)
		}
	case tea.KeyEsc:
		m = m.goBack()
	}
	return m, nil
}

func (m model) handleWifiManualSsidStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyBackspace:
		if len(m.manualSsid) > 0 {
			m.manualSsid = m.manualSsid[:len(m.manualSsid)-1]
		}
	case tea.KeyEnter:
		if m.manualSsid == "" {
			m.err = "SSID cannot be empty"
			return m, nil
		}
		m.err = ""
		m.substep = 2
	case tea.KeyEsc:
		m.substep = 0
		m.manualSsid = ""
		m.err = ""
	default:
		if len(msg.Runes) > 0 && msg.Runes[0] >= 32 {
			m.manualSsid += string(msg.Runes)
		}
	}
	return m, nil
}

func (m model) handleWifiPassStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyBackspace:
		if len(m.wifiPass) > 0 {
			m.wifiPass = m.wifiPass[:len(m.wifiPass)-1]
		}
	case tea.KeyEnter:
		m.err = ""
		device := getWifiDevice()
		var ssid string
		if m.manualWifi {
			ssid = m.manualSsid
		} else {
			ssid = m.ssids[m.selectedSsid]
		}
		m.substep = 3
		return m, connectWifi(device, ssid, m.wifiPass)
	case tea.KeyEsc:
		if m.manualWifi {
			m.substep = 1
			m.wifiPass = ""
			m.err = ""
		} else {
			m.substep = 0
			m.wifiPass = ""
			m.err = ""
		}
	default:
		if len(msg.Runes) > 0 && msg.Runes[0] >= 32 {
			m.wifiPass += string(msg.Runes)
		}
	}
	return m, nil
}

func (m model) handleUsernameStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyBackspace:
		if len(m.username) > 0 {
			m.username = m.username[:len(m.username)-1]
		}
	case tea.KeyEnter:
		if m.username == "" {
			m.err = "Username cannot be empty"
			return m, nil
		}
		if !regexp.MustCompile(`^[a-z_][a-z0-9_-]*[$]?$`).MatchString(m.username) {
			m.err = "Username must be lowercase alphanumeric with underscores or dashes"
			return m, nil
		}
		m = m.advanceStep()
	case tea.KeyEsc:
		m = m.goBack()
	default:
		if len(msg.Runes) > 0 && msg.Runes[0] >= 32 {
			m.username += string(msg.Runes)
		}
	}
	return m, nil
}

func (m model) handlePasswordStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyBackspace:
		if len(m.password) > 0 {
			m.password = m.password[:len(m.password)-1]
		}
	case tea.KeyEnter:
		if m.password == "" {
			m.err = "Password cannot be empty"
			return m, nil
		}
		if len(m.password) < 4 {
			m.err = "Password must be at least 4 characters"
			return m, nil
		}
		m.err = ""
		m.substep = 1
	case tea.KeyEsc:
		m = m.goBack()
	default:
		if len(msg.Runes) > 0 && msg.Runes[0] >= 32 {
			m.password += string(msg.Runes)
		}
	}
	return m, nil
}

func (m model) handleConfirmPasswordStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyBackspace:
		if len(m.confirmPass) > 0 {
			m.confirmPass = m.confirmPass[:len(m.confirmPass)-1]
		}
	case tea.KeyEnter:
		if m.confirmPass != m.password {
			m.err = "Passwords do not match"
			return m, nil
		}
		m.err = ""
		m.substep = 2
	case tea.KeyEsc:
		m.substep = 0
		m.confirmPass = ""
		m.err = ""
	default:
		if len(msg.Runes) > 0 && msg.Runes[0] >= 32 {
			m.confirmPass += string(msg.Runes)
		}
	}
	return m, nil
}

func (m model) handleLuksPassStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyBackspace:
		if len(m.luksPass) > 0 {
			m.luksPass = m.luksPass[:len(m.luksPass)-1]
		}
	case tea.KeyEnter:
		if m.luksPass == "" {
			m.err = "Passphrase cannot be empty"
			return m, nil
		}
		if len(m.luksPass) < 4 {
			m.err = "Passphrase must be at least 4 characters"
			return m, nil
		}
		m.err = ""
		m.substep = 3
	case tea.KeyEsc:
		m.substep = 1
		m.luksPass = ""
		m.err = ""
	default:
		if len(msg.Runes) > 0 && msg.Runes[0] >= 32 {
			m.luksPass += string(msg.Runes)
		}
	}
	return m, nil
}

func (m model) handleConfirmLuksStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyBackspace:
		if len(m.confirmLuks) > 0 {
			m.confirmLuks = m.confirmLuks[:len(m.confirmLuks)-1]
		}
	case tea.KeyEnter:
		if m.confirmLuks != m.luksPass {
			m.err = "Passphrases do not match"
			return m, nil
		}
		m.err = ""
		m.confirmLuks = ""
		m = m.advanceStep()
	case tea.KeyEsc:
		m.substep = 2
		m.confirmLuks = ""
		m.err = ""
	default:
		if len(msg.Runes) > 0 && msg.Runes[0] >= 32 {
			m.confirmLuks += string(msg.Runes)
		}
	}
	return m, nil
}

func (m model) handleHostnameStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyBackspace:
		if len(m.hostname) > 0 {
			m.hostname = m.hostname[:len(m.hostname)-1]
		}
	case tea.KeyEnter:
		if m.hostname == "" {
			m.hostname = "archlinux"
		}
		if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`).MatchString(m.hostname) {
			m.err = "Hostname must be alphanumeric with dashes or underscores"
			return m, nil
		}
		m = m.advanceStep()
	case tea.KeyEsc:
		m = m.goBack()
	default:
		if len(msg.Runes) > 0 && msg.Runes[0] >= 32 && msg.Runes[0] != ' ' {
			m.hostname += string(msg.Runes)
		}
	}
	return m, nil
}

func (m model) handleTimezoneStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.selectedTz > 0 {
			m.selectedTz--
		}
	case tea.KeyDown:
		if m.selectedTz < len(m.filteredTzs)-1 {
			m.selectedTz++
		}
	case tea.KeyEnter:
		if len(m.filteredTzs) > 0 {
			m.timezone = m.filteredTzs[m.selectedTz]
			m = m.advanceStep()
		}
	case tea.KeyEsc:
		m = m.goBack()
	default:
		// Filter typing for timezone
		if len(msg.Runes) > 0 && (unicode.IsLetter(msg.Runes[0]) || msg.Runes[0] == '/' || msg.Runes[0] == '_') {
			m.tzFilter += strings.ToLower(string(msg.Runes))
			m.filterTimezones()
		} else if msg.Type == tea.KeyBackspace && len(m.tzFilter) > 0 {
			m.tzFilter = m.tzFilter[:len(m.tzFilter)-1]
			m.filterTimezones()
		}
	}
	return m, nil
}

func (m *model) filterTimezones() {
	m.filteredTzs = nil
	for _, tz := range m.timezones {
		if strings.Contains(strings.ToLower(tz), m.tzFilter) {
			m.filteredTzs = append(m.filteredTzs, tz)
		}
	}
	if len(m.filteredTzs) > 0 {
		m.selectedTz = 0
	}
}

func (m model) handleDiskStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.selectedDisk > 0 {
			m.selectedDisk--
		}
	case tea.KeyDown:
		if m.selectedDisk < len(m.disks)-1 {
			m.selectedDisk++
		}
	case tea.KeyEnter:
		if len(m.disks) > 0 {
			m.disk = m.disks[m.selectedDisk].device
			m = m.advanceStep()
		}
	case tea.KeyEsc:
		m = m.goBack()
	}
	return m, nil
}

func (m model) handleConfirmStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.complete = true
		m.writeEnvFile()
		return m, tea.Quit
	case tea.KeyRunes:
		// Allow 1-8 to jump to edit specific steps
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case '1':
				m.editMode = true
				m.editReturnTo = stepConfirm
				m.step = stepKeyboard
				m.keyboardFilter = ""
			case '2':
				m.editMode = true
				m.editReturnTo = stepConfirm
				m.step = stepUsername
				m.username = ""
			case '3':
				m.editMode = true
				m.editReturnTo = stepConfirm
				m.step = stepPassword
				m.substep = 0
				m.password = ""
				m.confirmPass = ""
				m.luksPass = ""
				m.confirmLuks = ""
			case '4':
				m.editMode = true
				m.editReturnTo = stepConfirm
				m.step = stepPassword
				m.substep = 2
				m.luksPass = ""
				m.confirmLuks = ""
			case '5':
				m.editMode = true
				m.editReturnTo = stepConfirm
				m.step = stepHostname
			case '6':
				m.editMode = true
				m.editReturnTo = stepConfirm
				m.step = stepWifi
				m.substep = 0
				m.selectedSsid = 0
				m.wifiPass = ""
			case '7':
				m.editMode = true
				m.editReturnTo = stepConfirm
				m.step = stepTimezone
				m.tzFilter = ""
				m.filteredTzs = m.timezones
				m.selectedTz = 0
			case '8':
				m.editMode = true
				m.editReturnTo = stepConfirm
				m.step = stepDisk
			}
		}
	case tea.KeyEsc:
		// Go back to disk selection
		m.step = stepDisk
	}
	return m, nil
}

func (m model) View() string {
	if m.complete {
		return m.viewComplete()
	}

	var content string

	// Header with progress
	content += m.viewHeader()
	content += "\n"

	// Main card content
	var cardContent string
	switch m.step {
	case stepKeyboard:
		cardContent = m.viewKeyboardStep()
	case stepUsername:
		cardContent = m.viewUsernameStep()
	case stepPassword:
		switch m.substep {
		case 0:
			cardContent = m.viewPasswordStep()
		case 1:
			cardContent = m.viewConfirmPasswordStep()
		case 2:
			cardContent = m.viewLuksPassStep()
		case 3:
			cardContent = m.viewConfirmLuksStep()
		}
	case stepHostname:
		cardContent = m.viewHostnameStep()
	case stepWifi:
		if m.substep == 0 {
			cardContent = m.viewWifiSsidStep()
		} else if m.substep == 1 && m.manualWifi {
			cardContent = m.viewWifiManualSsidStep()
		} else if m.substep == 3 {
			cardContent = m.viewWifiConnectingStep()
		} else {
			cardContent = m.viewWifiPassStep()
		}
	case stepTimezone:
		cardContent = m.viewTimezoneStep()
	case stepDisk:
		cardContent = m.viewDiskStep()
	case stepConfirm:
		cardContent = m.viewConfirmStep()
	}

	// Apply card styling
	cardStyleToUse := cardStyle
	if m.step != stepConfirm {
		cardStyleToUse = cardActiveStyle
	}
	content += cardStyleToUse.Render(cardContent)

	// Center everything on screen
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m model) viewHeader() string {
	// KB • Usr • Pwd • LUKS • Host • WiFi • TZ • Disk • ✓
	steps := []string{"KB", "Usr", "Pwd", "LUKS", "Host", "WiFi", "TZ", "Disk", "✓"}
	var stepIndicators []string

	dividerStyle := lipgloss.NewStyle().Foreground(textMuted).Render("•")

	// Map model step+substep to header step index
	var headerIdx int
	switch m.step {
	case stepKeyboard:
		headerIdx = 0
	case stepUsername:
		headerIdx = 1
	case stepPassword:
		if m.substep < 2 {
			headerIdx = 2 // Pwd
		} else {
			headerIdx = 3 // LUKS
		}
	case stepHostname:
		headerIdx = 4
	case stepWifi:
		headerIdx = 5
	case stepTimezone:
		headerIdx = 6
	case stepDisk:
		headerIdx = 7
	case stepConfirm:
		headerIdx = 8
	}

	for i, step := range steps {
		var styled string
		if i < headerIdx {
			styled = stepDoneStyle.Render(step)
		} else if i == headerIdx {
			styled = stepActiveStyle.Render(step)
		} else {
			styled = stepTodoStyle.Render(step)
		}
		stepIndicators = append(stepIndicators, styled)
	}

	progressLine := strings.Join(stepIndicators, " "+dividerStyle+" ")

	headerContent := lipgloss.NewStyle().Foreground(brandPurple).Bold(true).Render("nomarchy installer") + "\n\n" + progressLine
	if m.editMode {
		headerContent = editBannerStyle.Render(" ✎ EDIT MODE ") + "\n\n" + progressLine
	}

	return headerStyle.Render(headerContent)
}

func (m model) viewKeyboardStep() string {
	var s string

	// Line 1: Filter (conditional) or empty
	if m.keyboardFilter != "" {
		s += labelStyle.Render("Filter: "+m.keyboardFilter+"_") + "\n"
	} else {
		s += "\n"
	}

	// Line 2: Title
	s += titleStyle.Render("Keyboard Layout") + "\n"

	// Line 3: Empty
	s += "\n"

	// Lines 4-11: List items (8 lines fixed)
	start := m.selectedKbd - 4
	if start < 0 {
		start = 0
	}
	end := start + 8
	if end > len(m.keyboards) {
		end = len(m.keyboards)
		start = end - 8
		if start < 0 {
			start = 0
		}
	}

	displayedCount := 0
	for i := start; i < end; i++ {
		kbd := m.keyboards[i]
		if i == m.selectedKbd {
			s += selectedStyle.Render(kbd.name) + "\n"
		} else {
			s += inputStyle.Render(kbd.name) + "\n"
		}
		displayedCount++
	}
	for i := displayedCount; i < 8; i++ {
		s += "\n"
	}

	// Line 12: Empty (symmetry)
	s += "\n"

	// Line 13: Empty
	s += "\n"

	// Line 14: Help
	s += helpStyle.Render("↑↓ navigate • type to filter • enter confirm • esc back")
	return s
}

func (m model) viewWifiSsidStep() string {
	var s string

	// Line 1: Empty
	s += "\n"

	// Line 2: Title
	s += titleStyle.Render("WiFi Network") + "\n"

	// Line 3: Empty
	s += "\n"

	// Lines 4-11: List items (8 lines fixed)
	start := m.selectedSsid - 4
	if start < 0 {
		start = 0
	}
	end := start + 8
	if end > len(m.ssids) {
		end = len(m.ssids)
		start = end - 8
		if start < 0 {
			start = 0
		}
	}

	displayedCount := 0
	for i := start; i < end; i++ {
		ssid := m.ssids[i]
		if i == m.selectedSsid {
			s += selectedStyle.Render(ssid) + "\n"
		} else {
			s += inputStyle.Render(ssid) + "\n"
		}
		displayedCount++
	}
	for i := displayedCount; i < 8; i++ {
		s += "\n"
	}

	// Line 12: Empty (symmetry)
	s += "\n"

	// Line 13: Empty
	s += "\n"

	// Line 14: Help
	refreshLabel := "r refresh"
	if m.scanning {
		refreshLabel = "r refresh " + spinnerFrames[m.spinnerFrame]
	}
	s += helpStyle.Render("↑↓ navigate • enter select • " + refreshLabel + " • esc skip")
	return s
}

func (m model) viewWifiManualSsidStep() string {
	var s string

	// Line 1: Empty
	s += "\n"

	// Line 2: Title
	s += titleStyle.Render("WiFi SSID") + "\n"

	// Line 3: Error (conditional) or empty
	if m.err != "" {
		s += errorStyle.Render(m.err) + "\n"
	} else {
		s += "\n"
	}

	// Lines 4-11: Input field (line 4), pad to 11
	s += inputActiveStyle.Render(m.manualSsid+"█") + "\n"
	s += "\n\n\n\n\n\n"

	// Line 12: Empty (symmetry)
	s += "\n"

	// Line 13: Description
	s += labelStyle.Render("Enter the WiFi network name") + "\n"

	// Line 14: Help
	s += helpStyle.Render("enter confirm • esc back")

	return s
}

func (m model) viewWifiPassStep() string {
	var s string

	// Line 1: Empty
	s += "\n"

	// Line 2: Title
	s += titleStyle.Render("WiFi Password") + "\n"

	// Line 3: Error (conditional) or empty
	if m.err != "" {
		s += errorStyle.Render(m.err) + "\n"
	} else {
		s += "\n"
	}

	// Lines 4-11: Input field (line 4), pad to 11
	hidden := strings.Repeat("•", len(m.wifiPass))
	s += inputActiveStyle.Render(hidden+"█") + "\n"
	s += "\n\n\n\n\n\n"

	// Line 12: Empty (symmetry)
	s += "\n"

	// Line 13: Description
	var ssid string
	if m.manualWifi {
		ssid = m.manualSsid
	} else {
		ssid = m.ssids[m.selectedSsid]
	}
	s += labelStyle.Render("Password for "+ssid) + "\n"

	// Line 14: Help
	s += helpStyle.Render("enter confirm • esc back")

	return s
}

func (m model) viewWifiConnectingStep() string {
	var s string

	// Line 1: Empty
	s += "\n"

	// Line 2: Title
	s += titleStyle.Render("WiFi Connection") + "\n"

	// Line 3: Empty
	s += "\n"

	// Lines 4-11: Message centered
	msg := m.message
	if msg == "" {
		msg = "Connecting..."
	}
	s += "\n\n\n" + successStyle.Render(msg) + "\n\n\n\n"

	// Line 12: Empty (symmetry)
	s += "\n"

	// Line 13: Empty
	s += "\n"

	// Line 14: Help
	s += helpStyle.Render("Please wait...")
	return s
}

func (m model) viewUsernameStep() string {
	var s string

	// Line 1: Empty
	s += "\n"

	// Line 2: Title
	s += titleStyle.Render("Username") + "\n"

	// Line 3: Error (conditional) or empty
	if m.err != "" {
		s += errorStyle.Render(m.err) + "\n"
	} else {
		s += "\n"
	}

	// Lines 4-11: Input field (line 4), pad to 11
	s += inputActiveStyle.Render(m.username+"█") + "\n"
	s += "\n\n\n\n\n\n"

	// Line 12: Empty (symmetry)
	s += "\n"

	// Line 13: Description
	s += labelStyle.Render("Lowercase letters, numbers, underscores, dashes") + "\n"

	// Line 14: Help
	if m.editMode {
		s += helpStyle.Render("enter save • esc cancel")
	} else {
		s += helpStyle.Render("enter confirm • esc back")
	}

	return s
}

func (m model) viewPasswordStep() string {
	var s string

	// Line 1: Empty
	s += "\n"

	// Line 2: Title
	s += titleStyle.Render("User Password") + "\n"

	// Line 3: Error (conditional) or empty
	if m.err != "" {
		s += errorStyle.Render(m.err) + "\n"
	} else {
		s += "\n"
	}

	// Lines 4-11: Input field (line 4), pad to 11
	hidden := strings.Repeat("•", len(m.password))
	s += inputActiveStyle.Render(hidden+"█") + "\n"
	s += "\n\n\n\n\n\n"

	// Line 12: Empty (symmetry)
	s += "\n"

	// Line 13: Description
	s += labelStyle.Render("Password for your user account") + "\n"

	// Line 14: Help
	if m.editMode {
		s += helpStyle.Render("enter confirm • esc cancel")
	} else {
		s += helpStyle.Render("enter confirm • esc back")
	}

	return s
}

func (m model) viewConfirmPasswordStep() string {
	var s string

	// Line 1: Empty
	s += "\n"

	// Line 2: Title
	s += titleStyle.Render("Confirm Password") + "\n"

	// Line 3: Error (conditional) or empty
	if m.err != "" {
		s += errorStyle.Render(m.err) + "\n"
	} else {
		s += "\n"
	}

	// Lines 4-11: Input field (line 4), pad to 11
	hidden := strings.Repeat("•", len(m.confirmPass))
	s += inputActiveStyle.Render(hidden+"█") + "\n"
	s += "\n\n\n\n\n\n"

	// Line 12: Empty (symmetry)
	s += "\n"

	// Line 13: Empty
	s += "\n"

	// Line 14: Help
	s += helpStyle.Render("enter confirm • esc back")
	return s
}

func (m model) viewLuksPassStep() string {
	var s string

	// Line 1: Empty
	s += "\n"

	// Line 2: Title
	s += titleStyle.Render("LUKS Passphrase") + "\n"

	// Line 3: Error (conditional) or empty
	if m.err != "" {
		s += errorStyle.Render(m.err) + "\n"
	} else {
		s += "\n"
	}

	// Lines 4-11: Input field (line 4), pad to 11
	hidden := strings.Repeat("•", len(m.luksPass))
	s += inputActiveStyle.Render(hidden+"█") + "\n"
	s += "\n\n\n\n\n\n"

	// Line 12: Empty (symmetry)
	s += "\n"

	// Line 13: Description
	s += labelStyle.Render("Full-disk encryption passphrase (entered at boot)") + "\n"

	// Line 14: Help
	if m.editMode {
		s += helpStyle.Render("enter confirm • esc cancel")
	} else {
		s += helpStyle.Render("enter confirm • esc back")
	}

	return s
}

func (m model) viewConfirmLuksStep() string {
	var s string

	// Line 1: Empty
	s += "\n"

	// Line 2: Title
	s += titleStyle.Render("Confirm LUKS Passphrase") + "\n"

	// Line 3: Error (conditional) or empty
	if m.err != "" {
		s += errorStyle.Render(m.err) + "\n"
	} else {
		s += "\n"
	}

	// Lines 4-11: Input field (line 4), pad to 11
	hidden := strings.Repeat("•", len(m.confirmLuks))
	s += inputActiveStyle.Render(hidden+"█") + "\n"
	s += "\n\n\n\n\n\n"

	// Line 12: Empty (symmetry)
	s += "\n"

	// Line 13: Empty
	s += "\n"

	// Line 14: Help
	s += helpStyle.Render("enter confirm • esc back")
	return s
}

func (m model) viewHostnameStep() string {
	var s string

	// Line 1: Empty
	s += "\n"

	// Line 2: Title
	s += titleStyle.Render("Hostname") + "\n"

	// Line 3: Error (conditional) or empty
	if m.err != "" {
		s += errorStyle.Render(m.err) + "\n"
	} else {
		s += "\n"
	}

	// Lines 4-11: Input field (line 4), pad to 11
	s += inputActiveStyle.Render(m.hostname+"█") + "\n"
	s += "\n\n\n\n\n\n"

	// Line 12: Empty (symmetry)
	s += "\n"

	// Line 13: Description
	s += labelStyle.Render("Name for this computer on the network") + "\n"

	// Line 14: Help
	if m.editMode {
		s += helpStyle.Render("enter save • esc cancel")
	} else {
		s += helpStyle.Render("enter confirm (default: archlinux) • esc back")
	}

	return s
}

func (m model) viewTimezoneStep() string {
	var s string

	// Line 1: Filter (conditional)
	if m.tzFilter != "" {
		s += labelStyle.Render("Filter: "+m.tzFilter+"_") + "\n"
	} else {
		s += "\n"
	}

	// Line 2: Title
	s += titleStyle.Render("Timezone") + "\n"

	// Line 3: Empty
	s += "\n"

	// Lines 4-11: List items (8 lines fixed)
	start := m.selectedTz - 4
	if start < 0 {
		start = 0
	}
	end := start + 8
	if end > len(m.filteredTzs) {
		end = len(m.filteredTzs)
		start = end - 8
		if start < 0 {
			start = 0
		}
	}

	displayedCount := 0
	for i := start; i < end && i < len(m.filteredTzs); i++ {
		tz := m.filteredTzs[i]
		if i == m.selectedTz {
			s += selectedStyle.Render(tz) + "\n"
		} else {
			s += inputStyle.Render(tz) + "\n"
		}
		displayedCount++
	}
	for i := displayedCount; i < 8; i++ {
		s += "\n"
	}

	// Line 12: Empty (symmetry)
	s += "\n"

	// Line 13: Empty
	s += "\n"

	// Line 14: Help
	s += helpStyle.Render("↑↓ navigate • type to filter • enter confirm • esc back")
	return s
}

func (m model) viewDiskStep() string {
	var s string

	// Line 1: Empty
	s += "\n"

	// Line 2: Title
	s += titleStyle.Render("Install Disk") + "\n"

	// Line 3: Warning (always shown)
	s += warningStyle.Render("⚠ All data on selected disk will be erased!") + "\n"

	// Lines 4-11: List items (8 lines fixed)
	if len(m.disks) == 0 {
		s += errorStyle.Render("No disks found!") + "\n"
		s += "\n\n\n\n\n"
	} else {
		start := m.selectedDisk - 4
		if start < 0 {
			start = 0
		}
		end := start + 8
		if end > len(m.disks) {
			end = len(m.disks)
			start = end - 8
			if start < 0 {
				start = 0
			}
		}

		displayedCount := 0
		for i := start; i < end; i++ {
			disk := m.disks[i]
			if i == m.selectedDisk {
				s += selectedStyle.Render(disk.name) + "\n"
			} else {
				s += inputStyle.Render(disk.name) + "\n"
			}
			displayedCount++
		}
		for i := displayedCount; i < 8; i++ {
			s += "\n"
		}
	}

	// Line 12: Empty (symmetry)
	s += "\n"

	// Line 13: Empty
	s += "\n"

	// Line 14: Help
	s += helpStyle.Render("↑↓ navigate • enter confirm • esc back")
	return s
}

func (m model) viewConfirmStep() string {
	var s string

	// Line 1: Empty
	s += "\n"

	// Line 2: Title
	s += titleStyle.Render("Review Configuration") + "\n"

	// Line 3: Instruction
	s += labelStyle.Render("Press a number [1-8] to edit, or Enter to install:") + "\n"

	// Lines 4-11: Configuration summary (8 rows)
	kbdName := m.keyboards[m.selectedKbd].name
	wifiName := ""
	if len(m.ssids) > 0 && m.selectedSsid < len(m.ssids) {
		wifiName = m.ssids[m.selectedSsid]
	}
	tzName := m.timezone
	if tzName == "" && len(m.filteredTzs) > 0 && m.selectedTz < len(m.filteredTzs) {
		tzName = m.filteredTzs[m.selectedTz]
	}
	diskName := m.disk
	if diskName == "" && len(m.disks) > 0 && m.selectedDisk < len(m.disks) {
		diskName = m.disks[m.selectedDisk].device
	}

	rows := []struct {
		num   int
		label string
		value string
	}{
		{1, "Keyboard", kbdName},
		{2, "Username", m.username},
		{3, "Password", strings.Repeat("•", len(m.password))},
		{4, "LUKS", strings.Repeat("•", len(m.luksPass))},
		{5, "Hostname", m.hostname},
		{6, "WiFi", wifiName},
		{7, "Timezone", tzName},
		{8, "Disk", diskName},
	}

	for _, row := range rows {
		num := lipgloss.NewStyle().Foreground(brandPurple).Bold(true).Render(fmt.Sprintf("[%d]", row.num))
		label := labelStyle.Render(fmt.Sprintf("%-10s", row.label))
		value := lipgloss.NewStyle().Foreground(textPrimary).Render(row.value)
		s += fmt.Sprintf("%s %s %s\n", num, label, value)
	}

	// Line 12: Description
	s += lipgloss.NewStyle().
		Foreground(textSuccess).
		Bold(true).
		Render("Press [Enter] to begin installation") + "\n"

	// Line 13: Help
	s += helpStyle.Render("Press [Esc] to go back • [Q] to quit without saving")

	return s
}

func (m model) viewComplete() string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		cardActiveStyle.Render(
			successStyle.Bold(true).Render("✓ Configuration saved!")+"\n\n"+
				labelStyle.Render("Proceeding with installation...")+"\n"+
				labelStyle.Render("You can close this window."),
		),
	)
}

// bashQuote wraps s in single quotes, escaping any single quotes within.
func bashQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func (m model) writeEnvFile() {
	var ssid, wifiPass string
	if m.wifiPass != "" {
		if m.manualWifi {
			ssid = m.manualSsid
		} else if m.selectedSsid < len(m.ssids) {
			ssid = m.ssids[m.selectedSsid]
		}
		wifiPass = m.wifiPass
	}

	lines := []string{
		"INSTALL_KB_LAYOUT=" + bashQuote(m.keyboards[m.selectedKbd].layout),
		"INSTALL_USER=" + bashQuote(m.username),
		"USER_PASSWORD=" + bashQuote(m.password),
		"LUKS_PASSPHRASE=" + bashQuote(m.luksPass),
		"INSTALL_HOSTNAME=" + bashQuote(m.hostname),
		"INSTALL_DISK=" + bashQuote(m.disk),
		"INSTALL_TIMEZONE=" + bashQuote(m.timezone),
	}
	if ssid != "" {
		lines = append(lines, "WIFI_SSID="+bashQuote(ssid))
		lines = append(lines, "WIFI_PASSWORD="+bashQuote(wifiPass))
	}

	content := strings.Join(lines, "\n") + "\n"
	os.WriteFile("/tmp/nomarchy-install.env", []byte(content), 0600)
}

func main() {
	p := tea.NewProgram(newModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
