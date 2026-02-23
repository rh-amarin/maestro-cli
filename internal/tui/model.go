package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	sigyaml "sigs.k8s.io/yaml"

	"github.com/openshift-hyperfleet/maestro-cli/internal/maestro"
)

// ─── Screen / panel states ────────────────────────────────────────────────────

type screenState int

const (
	screenConnect screenState = iota
	screenMain
)

type focusedPanel int

const (
	panelConsumers focusedPanel = iota
	panelManifests
	panelDetail
)

type detailViewMode int

const (
	viewModeFormatted detailViewMode = iota
	viewModeJSON
	viewModeYAML
)

func (m detailViewMode) String() string {
	switch m {
	case viewModeFormatted:
		return "Formatted"
	case viewModeJSON:
		return "JSON"
	case viewModeYAML:
		return "YAML"
	default:
		return "Formatted"
	}
}

func (m detailViewMode) next() detailViewMode {
	return (m + 1) % 3
}

// ─── Message types ────────────────────────────────────────────────────────────

type errMsg struct{ err error }
type connectedMsg struct {
	client    *maestro.Client
	consumers []maestro.ConsumerInfo
}
type consumersLoadedMsg struct{ consumers []maestro.ConsumerInfo }
type manifestsLoadedMsg struct {
	manifests []maestro.ResourceBundleSummary
}
type detailLoadedMsg struct {
	detail   *maestro.ManifestWorkDetails
	jsonData string // syntax-colored
	yamlData string // syntax-colored
	rawJSON  string // plain, for clipboard
	rawYAML  string // plain, for clipboard
}
type consumerCreatedMsg struct{ consumer maestro.ConsumerInfo }
type consumerDeletedMsg struct{}
type manifestDeletedMsg struct{}
type watchTickMsg time.Time
type spinnerTickMsg time.Time
type clipboardMsg struct{ err error }

// searchMatch records the position of one search hit within the detail content.
type searchMatch struct {
	line  int // 0-indexed line number in the rendered content
	start int // visual char offset within that line (plain text)
	end   int // exclusive end
}

// ─── Model ────────────────────────────────────────────────────────────────────

// Model is the Bubble Tea application model.
type Model struct {
	width, height int
	screen        screenState

	// Connect form
	connectInputs   [2]textinput.Model
	connectInsecure bool
	connectFocusIdx int
	connectLoading  bool

	// Main
	client       *maestro.Client
	clientConfig maestro.ClientConfig
	focused      focusedPanel

	// Consumers
	consumers      []maestro.ConsumerInfo
	consumerCursor int
	consumerOffset int

	// ManifestWorks
	manifests      []maestro.ResourceBundleSummary
	manifestCursor int
	manifestOffset int
	filterInput    textinput.Model
	filtering      bool
	filterText     string

	// Detail
	viewport        viewport.Model
	detailContent   string // rendered content for current view mode
	detailFormatted string // formatted (pretty) view
	detailJSON      string // syntax-colored JSON
	detailYAML      string // syntax-colored YAML
	detailRawJSON   string // plain JSON (for clipboard)
	detailRawYAML   string // plain YAML (for clipboard)
	detailViewMode  detailViewMode

	// Search within detail viewport
	searchInput   textinput.Model
	searching     bool   // search bar is active (user is typing)
	searchText    string // current query
	searchMatches []searchMatch
	searchCurrent int // index into searchMatches

	// Watch
	watching bool

	// Modals — create consumer
	showCreateConsumer bool
	createInput        textinput.Model

	// Modals — confirm delete
	showConfirm bool
	confirmKind string // "consumer" | "manifest"
	confirmID   string
	confirmName string
	confirmMsg  string

	// Status
	loading    bool
	statusMsg  string
	errMsg2    string // renamed to avoid clash with errMsg type
	spinnerIdx int
}

// New creates a new Model pre-populated from the given ClientConfig.
func New(config maestro.ClientConfig) Model {
	// Endpoint input
	ep := textinput.New()
	ep.Placeholder = "http://localhost:8000"
	ep.SetValue(config.HTTPEndpoint)
	ep.Focus()
	ep.Width = 40

	// Token input
	tok := textinput.New()
	tok.Placeholder = "Bearer token (optional)"
	tok.SetValue(config.GRPCClientToken)
	tok.EchoMode = textinput.EchoPassword
	tok.EchoCharacter = '•'
	tok.Width = 40

	// Filter input
	fi := textinput.New()
	fi.Placeholder = "filter..."
	fi.Width = 30

	// Create consumer input
	ci := textinput.New()
	ci.Placeholder = "consumer name"
	ci.Width = 30

	// Detail search input
	si := textinput.New()
	si.Placeholder = "search..."
	si.Width = 30
	si.Prompt = "/ "

	vp := viewport.New(60, 20)
	vp.Style = lipgloss.NewStyle()

	return Model{
		screen:        screenConnect,
		connectInputs: [2]textinput.Model{ep, tok},
		clientConfig:  config,
		focused:       panelConsumers,
		filterInput:   fi,
		createInput:   ci,
		searchInput:   si,
		viewport:      vp,
	}
}

// ─── Init ─────────────────────────────────────────────────────────────────────

// Init implements tea.Model. It starts the spinner and text-input blink ticks.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		spinnerTick(),
	)
}

// ─── Update ───────────────────────────────────────────────────────────────────

// Update implements tea.Model. It routes messages to the appropriate handler.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// ── 1. Always forward every message to the active sub-component first.
	// This lets text inputs receive character keys, blink ticks, etc. before
	// our own key routing runs and potentially consumes the event.
	switch m.screen {
	case screenConnect:
		if m.connectFocusIdx < 2 {
			updated, cmd := m.connectInputs[m.connectFocusIdx].Update(msg)
			m.connectInputs[m.connectFocusIdx] = updated
			cmds = append(cmds, cmd)
		}
	case screenMain:
		switch {
		case m.showCreateConsumer:
			updated, cmd := m.createInput.Update(msg)
			m.createInput = updated
			cmds = append(cmds, cmd)
		case m.filtering:
			prevFilter := m.filterText
			updated, cmd := m.filterInput.Update(msg)
			m.filterInput = updated
			m.filterText = m.filterInput.Value()
			if m.filterText != prevFilter {
				m.manifestCursor = 0
				m.manifestOffset = 0
			}
			cmds = append(cmds, cmd)
		case m.searching:
			prevText := m.searchText
			updated, cmd := m.searchInput.Update(msg)
			m.searchInput = updated
			m.searchText = m.searchInput.Value()
			if m.searchText != prevText {
				m.rebuildSearch()
			}
			cmds = append(cmds, cmd)
		case m.focused == panelDetail:
			updated, cmd := m.viewport.Update(msg)
			m.viewport = updated
			cmds = append(cmds, cmd)
		}
	}

	// ── 2. Handle specific message types.
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpW, vpH := m.detailPanelDims()
		m.viewport.Width = vpW - 4
		m.viewport.Height = vpH - 4
		m.viewport.SetContent(m.detailContent)

	case spinnerTickMsg:
		if m.loading || m.connectLoading {
			m.spinnerIdx = (m.spinnerIdx + 1) % len(spinnerFrames)
			cmds = append(cmds, spinnerTick())
		}

	case errMsg:
		m.loading = false
		m.connectLoading = false
		m.errMsg2 = msg.err.Error()
		m.statusMsg = ""

	case connectedMsg:
		m.client = msg.client
		m.consumers = msg.consumers
		m.consumerCursor = 0
		m.consumerOffset = 0
		m.screen = screenMain
		m.connectLoading = false
		m.loading = false
		m.statusMsg = fmt.Sprintf("Connected — %d consumer(s)", len(m.consumers))
		m.errMsg2 = ""
		if len(m.consumers) > 0 {
			// With a single consumer skip the consumers panel and land on manifests
			if len(m.consumers) == 1 {
				m.focused = panelManifests
			}
			cmds = append(cmds, m.loadManifests(m.consumers[0].Name))
		}

	case consumersLoadedMsg:
		m.consumers = msg.consumers
		m.consumerCursor = 0
		m.consumerOffset = 0
		m.loading = false
		m.statusMsg = fmt.Sprintf("%d consumer(s)", len(m.consumers))

	case manifestsLoadedMsg:
		m.manifests = msg.manifests
		m.manifestCursor = 0
		m.manifestOffset = 0
		m.loading = false
		if len(m.manifests) > 0 {
			cmds = append(cmds, m.loadDetail(m.manifests[0]))
		}

	case detailLoadedMsg:
		m.loading = false
		m.detailFormatted = renderDetail(msg.detail)
		m.detailJSON = msg.jsonData
		m.detailYAML = msg.yamlData
		m.detailRawJSON = msg.rawJSON
		m.detailRawYAML = msg.rawYAML
		m.detailContent = m.activeDetailContent()
		if m.searchText != "" {
			m.rebuildSearch()
		} else {
			m.viewport.SetContent(m.detailContent)
			m.viewport.GotoTop()
		}
		if m.watching {
			cmds = append(cmds, watchTick())
		}

	case watchTickMsg:
		if m.watching && m.client != nil {
			selected := m.selectedManifest()
			if selected != nil {
				cmds = append(cmds, m.loadDetail(*selected))
			}
		}

	case consumerCreatedMsg:
		m.loading = false
		m.showCreateConsumer = false
		m.createInput.SetValue("")
		m.statusMsg = fmt.Sprintf("Consumer %q created", msg.consumer.Name)
		cmds = append(cmds, m.reloadConsumers())

	case consumerDeletedMsg:
		m.loading = false
		m.showConfirm = false
		m.statusMsg = "Consumer deleted"
		m.manifests = nil
		m.detailContent = ""
		m.viewport.SetContent("")
		cmds = append(cmds, m.reloadConsumers())

	case manifestDeletedMsg:
		m.loading = false
		m.showConfirm = false
		m.statusMsg = "ManifestWork deleted"
		m.detailContent = ""
		m.viewport.SetContent("")
		if len(m.consumers) > 0 {
			cmds = append(cmds, m.loadManifests(m.consumers[m.consumerCursor].Name))
		}

	case clipboardMsg:
		if msg.err != nil {
			m.statusMsg = ""
			m.errMsg2 = "clipboard: " + msg.err.Error()
		} else {
			m.errMsg2 = ""
			m.statusMsg = "Copied to clipboard!"
		}

	case tea.MouseMsg:
		newM, cmd := m.handleMouse(msg)
		m = newM.(Model)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tea.KeyMsg:
		// Global quit — always wins.
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		// Route special keys to the appropriate handler.
		// Handlers return a new model + optional cmd; we merge the cmd into cmds.
		var newM tea.Model
		var cmd tea.Cmd

		switch m.screen {
		case screenConnect:
			newM, cmd = m.handleConnectKey(msg)
		case screenMain:
			switch {
			case m.showCreateConsumer:
				newM, cmd = m.handleCreateConsumerKey(msg)
			case m.showConfirm:
				newM, cmd = m.handleConfirmKey(msg)
			default:
				newM, cmd = m.handleMainKey(msg)
			}
		default:
			newM = m
		}

		m = newM.(Model)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// ─── Key handlers ─────────────────────────────────────────────────────────────

func (m Model) handleConnectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type { //nolint:exhaustive
	case tea.KeyTab:
		m.connectFocusIdx = (m.connectFocusIdx + 1) % 4
		m.syncConnectFocus()
	case tea.KeyShiftTab:
		m.connectFocusIdx = (m.connectFocusIdx + 3) % 4
		m.syncConnectFocus()
	case tea.KeySpace:
		// Toggle insecure when focused on it (idx 2)
		if m.connectFocusIdx == 2 {
			m.connectInsecure = !m.connectInsecure
		}
	case tea.KeyEnter:
		if m.connectFocusIdx == 3 || m.connectFocusIdx == 1 {
			return m.doConnect()
		}
		// advance field
		m.connectFocusIdx = (m.connectFocusIdx + 1) % 4
		m.syncConnectFocus()
	}
	return m, nil
}

func (m *Model) syncConnectFocus() {
	for i := range m.connectInputs {
		m.connectInputs[i].Blur()
	}
	if m.connectFocusIdx < 2 {
		m.connectInputs[m.connectFocusIdx].Focus()
	}
}

func (m Model) doConnect() (tea.Model, tea.Cmd) {
	m.connectLoading = true
	m.errMsg2 = ""
	m.clientConfig.HTTPEndpoint = m.connectInputs[0].Value()
	m.clientConfig.GRPCClientToken = m.connectInputs[1].Value()
	m.clientConfig.GRPCInsecure = m.connectInsecure
	return m, tea.Batch(spinnerTick(), connectCmd(m.clientConfig))
}

func (m Model) handleCreateConsumerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type { //nolint:exhaustive
	case tea.KeyEscape:
		m.showCreateConsumer = false
		m.createInput.SetValue("")
	case tea.KeyEnter:
		name := strings.TrimSpace(m.createInput.Value())
		if name == "" {
			return m, nil
		}
		m.loading = true
		m.errMsg2 = ""
		return m, tea.Batch(spinnerTick(), m.createConsumerCmd(name))
	}
	return m, nil
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape:
		m.showConfirm = false
	case msg.String() == "y" || msg.String() == "Y":
		m.loading = true
		m.showConfirm = false
		m.errMsg2 = ""
		switch m.confirmKind {
		case "consumer":
			return m, tea.Batch(spinnerTick(), m.deleteConsumerCmd(m.confirmID))
		case "manifest":
			return m, tea.Batch(spinnerTick(), m.deleteManifestCmd(m.confirmID))
		}
	}
	return m, nil
}

func (m Model) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.focused {
	case panelConsumers:
		return m.handleConsumersKey(msg)
	case panelManifests:
		return m.handleManifestsKey(msg)
	case panelDetail:
		return m.handleDetailKey(msg)
	}
	return m, nil
}

func (m Model) handleConsumersKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyTab:
		m.focused = panelManifests
	case msg.Type == tea.KeyShiftTab:
		m.focused = panelDetail
	case msg.String() == "up" || msg.String() == "k":
		if m.consumerCursor > 0 {
			m.consumerCursor--
		}
	case msg.String() == "down" || msg.String() == "j":
		if m.consumerCursor < len(m.consumers)-1 {
			m.consumerCursor++
		}
	case msg.Type == tea.KeyEnter:
		if len(m.consumers) > 0 {
			m.loading = true
			m.manifests = nil
			m.detailContent = ""
			m.viewport.SetContent("")
			return m, tea.Batch(spinnerTick(), m.loadManifests(m.consumers[m.consumerCursor].Name))
		}
	case msg.String() == "n":
		m.showCreateConsumer = true
		m.createInput.Focus()
		m.createInput.SetValue("")
	case msg.String() == "d":
		if len(m.consumers) > 0 {
			c := m.consumers[m.consumerCursor]
			m.showConfirm = true
			m.confirmKind = "consumer"
			m.confirmID = c.ID
			m.confirmName = c.Name
			m.confirmMsg = fmt.Sprintf("Delete consumer %q?", c.Name)
		}
	case msg.String() == "r":
		m.loading = true
		return m, tea.Batch(spinnerTick(), m.reloadConsumers())
	case msg.String() == "y":
		return m, m.copyToClipboardCmd()
	}
	return m, nil
}

func (m Model) handleManifestsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		switch msg.Type { //nolint:exhaustive
		case tea.KeyEscape:
			m.filtering = false
			m.filterText = ""
			m.filterInput.SetValue("")
			m.filterInput.Blur()
			m.manifestCursor = 0
			m.manifestOffset = 0
		case tea.KeyEnter:
			m.filtering = false
			m.filterInput.Blur()
		}
		return m, nil
	}

	switch {
	case msg.Type == tea.KeyTab:
		m.focused = panelDetail
	case msg.Type == tea.KeyShiftTab:
		m.focused = panelConsumers
	case msg.String() == "up" || msg.String() == "k":
		visible := m.filteredManifests()
		if m.manifestCursor > 0 {
			m.manifestCursor--
			if m.manifestCursor < m.manifestOffset {
				m.manifestOffset = m.manifestCursor
			}
			if len(visible) > 0 {
				return m, m.loadDetail(visible[m.manifestCursor])
			}
		}
	case msg.String() == "down" || msg.String() == "j":
		visible := m.filteredManifests()
		if m.manifestCursor < len(visible)-1 {
			m.manifestCursor++
			return m, m.loadDetail(visible[m.manifestCursor])
		}
	case msg.String() == "/":
		m.filtering = true
		m.filterInput.Focus()
	case msg.String() == "w":
		m.watching = !m.watching
		if m.watching {
			m.statusMsg = "Watch mode ON"
			return m, watchTick()
		}
		m.statusMsg = "Watch mode OFF"
	case msg.String() == "v":
		m.cycleDetailViewMode()
	case msg.String() == "d":
		visible := m.filteredManifests()
		if len(visible) > 0 {
			mw := visible[m.manifestCursor]
			m.showConfirm = true
			m.confirmKind = "manifest"
			m.confirmID = mw.ID
			m.confirmName = mw.Name
			m.confirmMsg = fmt.Sprintf("Delete ManifestWork %q?", mw.Name)
		}
	case msg.String() == "r":
		if len(m.consumers) > 0 {
			m.loading = true
			return m, tea.Batch(spinnerTick(), m.loadManifests(m.consumers[m.consumerCursor].Name))
		}
	case msg.String() == "y":
		return m, m.copyToClipboardCmd()
	}
	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Search mode: Esc closes, Enter advances to next match.
	if m.searching {
		switch msg.Type { //nolint:exhaustive
		case tea.KeyEscape:
			m.clearSearch()
		case tea.KeyEnter:
			m.nextSearchMatch()
		}
		return m, nil
	}

	switch {
	case msg.Type == tea.KeyTab:
		m.focused = panelConsumers
	case msg.Type == tea.KeyShiftTab:
		m.focused = panelManifests
	case msg.String() == "/":
		m.searching = true
		m.searchInput.Focus()
	case msg.String() == "n":
		m.nextSearchMatch()
	case msg.String() == "N":
		m.prevSearchMatch()
	case msg.String() == "w":
		m.watching = !m.watching
		if m.watching {
			m.statusMsg = "Watch mode ON"
			return m, watchTick()
		}
		m.statusMsg = "Watch mode OFF"
	case msg.String() == "v":
		m.cycleDetailViewMode()
	case msg.String() == "y":
		return m, m.copyToClipboardCmd()
	case msg.String() == "r":
		selected := m.selectedManifest()
		if selected != nil {
			m.loading = true
			return m, tea.Batch(spinnerTick(), m.loadDetail(*selected))
		}
	default:
		updated, cmd := m.viewport.Update(msg)
		m.viewport = updated
		return m, cmd
	}
	return m, nil
}

// ─── Commands ─────────────────────────────────────────────────────────────────

func connectCmd(cfg maestro.ClientConfig) tea.Cmd {
	return func() tea.Msg {
		client, err := maestro.NewHTTPClient(cfg)
		if err != nil {
			return errMsg{err}
		}
		consumers, err := client.ListConsumersWithDetails(context.Background())
		if err != nil {
			return errMsg{err}
		}
		return connectedMsg{client: client, consumers: consumers}
	}
}

func (m Model) reloadConsumers() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		consumers, err := client.ListConsumersWithDetails(context.Background())
		if err != nil {
			return errMsg{err}
		}
		return consumersLoadedMsg{consumers: consumers}
	}
}

func (m Model) loadManifests(consumerName string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		manifests, err := client.ListManifestWorksHTTP(context.Background(), consumerName)
		if err != nil {
			return errMsg{err}
		}
		return manifestsLoadedMsg{manifests: manifests}
	}
}

func (m Model) loadDetail(mw maestro.ResourceBundleSummary) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		rb, err := client.GetResourceBundleHTTP(context.Background(), mw.ID)
		if err != nil {
			return errMsg{err}
		}
		detail := maestro.ResourceBundleToDetails(rb, mw.ConsumerName)

		// Build raw map for JSON/YAML rendering
		raw := maestro.ResourceBundleToRawMap(rb, mw.ConsumerName)

		rawJSON, rawYAML := "", ""
		jsonStr, yamlStr := "", ""

		if jsonBytes, e := json.MarshalIndent(raw, "", "  "); e == nil {
			rawJSON = string(jsonBytes)
			jsonStr = colorizeJSON(rawJSON)
		}
		if yamlBytes, e := sigyaml.Marshal(raw); e == nil {
			rawYAML = string(yamlBytes)
			yamlStr = colorizeYAML(rawYAML)
		}

		return detailLoadedMsg{
			detail:   detail,
			jsonData: jsonStr,
			yamlData: yamlStr,
			rawJSON:  rawJSON,
			rawYAML:  rawYAML,
		}
	}
}

func (m Model) createConsumerCmd(name string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		info, err := client.CreateConsumer(context.Background(), name)
		if err != nil {
			return errMsg{err}
		}
		return consumerCreatedMsg{consumer: *info}
	}
}

func (m Model) deleteConsumerCmd(id string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := client.DeleteConsumer(context.Background(), id)
		if err != nil {
			return errMsg{err}
		}
		return consumerDeletedMsg{}
	}
}

func (m Model) deleteManifestCmd(id string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := client.DeleteResourceBundleByID(context.Background(), id)
		if err != nil {
			return errMsg{err}
		}
		return manifestDeletedMsg{}
	}
}

var ansiEscRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes terminal escape sequences from s, producing plain text.
func stripANSI(s string) string {
	return ansiEscRe.ReplaceAllString(s, "")
}

// clipboardContent returns the text that should be written to the clipboard for
// the current view mode.  JSON/YAML modes use the pre-built raw (uncolored)
// strings; formatted mode strips ANSI from the rendered view.
func (m Model) clipboardContent() string {
	switch m.detailViewMode {
	case viewModeJSON:
		if m.detailRawJSON != "" {
			return m.detailRawJSON
		}
	case viewModeYAML:
		if m.detailRawYAML != "" {
			return m.detailRawYAML
		}
	case viewModeFormatted:
		// handled below
	}
	return stripANSI(m.detailFormatted)
}

func (m Model) copyToClipboardCmd() tea.Cmd {
	content := m.clipboardContent()
	return func() tea.Msg {
		err := clipboard.WriteAll(content)
		return clipboardMsg{err: err}
	}
}

// ─── Mouse handler ────────────────────────────────────────────────────────────

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.screen != screenMain {
		return m, nil
	}

	leftW := int(float64(m.width) * 0.40)
	totalH := m.height - 1
	consumerH := int(float64(totalH) * 0.40)

	x, y := msg.X, msg.Y

	switch msg.Button { //nolint:exhaustive
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		// Determine which panel was clicked and act accordingly.
		if x < leftW {
			if y < consumerH {
				return m.mouseClickConsumer(y)
			}
			return m.mouseClickManifest(y, consumerH)
		}
		// Clicked the detail panel — just focus it.
		m.focused = panelDetail

	case tea.MouseButtonWheelUp:
		if x < leftW {
			if y < consumerH {
				if m.consumerCursor > 0 {
					m.consumerCursor--
				}
			} else {
				if m.manifestCursor > 0 {
					m.manifestCursor--
					if sel := m.selectedManifest(); sel != nil {
						return m, m.loadDetail(*sel)
					}
				}
			}
		} else {
			m.viewport.ScrollUp(3)
		}

	case tea.MouseButtonWheelDown:
		if x < leftW {
			if y < consumerH {
				if m.consumerCursor < len(m.consumers)-1 {
					m.consumerCursor++
				}
			} else {
				visible := m.filteredManifests()
				if m.manifestCursor < len(visible)-1 {
					m.manifestCursor++
					if sel := m.selectedManifest(); sel != nil {
						return m, m.loadDetail(*sel)
					}
				}
			}
		} else {
			m.viewport.ScrollDown(3)
		}
	}

	return m, nil
}

func (m Model) mouseClickConsumer(y int) (tea.Model, tea.Cmd) {
	// Content starts after: border-top(1) + title(1) = row 2
	const headerRows = 2
	itemY := y - headerRows
	if itemY < 0 {
		m.focused = panelConsumers
		return m, nil
	}
	idx := itemY + m.consumerOffset
	if idx >= len(m.consumers) {
		m.focused = panelConsumers
		return m, nil
	}
	m.focused = panelConsumers
	m.consumerCursor = idx
	m.loading = true
	m.manifests = nil
	m.detailContent = ""
	m.viewport.SetContent("")
	return m, tea.Batch(spinnerTick(), m.loadManifests(m.consumers[idx].Name))
}

func (m Model) mouseClickManifest(y, consumerH int) (tea.Model, tea.Cmd) {
	// Content starts after: panelTop + border-top(1) + title(1) + filter(1) = +3
	const headerRows = 3
	panelY := y - consumerH
	itemY := panelY - headerRows
	if itemY < 0 {
		m.focused = panelManifests
		return m, nil
	}
	visible := m.filteredManifests()
	idx := itemY + m.manifestOffset
	if idx >= len(visible) {
		m.focused = panelManifests
		return m, nil
	}
	m.focused = panelManifests
	m.manifestCursor = idx
	return m, m.loadDetail(visible[idx])
}

// ─── Tick commands ────────────────────────────────────────────────────────────

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTickMsg(t)
	})
}

func watchTick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return watchTickMsg(t)
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// cycleDetailViewMode advances the view mode and refreshes the viewport.
func (m *Model) cycleDetailViewMode() {
	m.detailViewMode = m.detailViewMode.next()
	m.detailContent = m.activeDetailContent()
	if m.searchText != "" {
		m.rebuildSearch()
	} else {
		m.viewport.SetContent(m.detailContent)
		m.viewport.GotoTop()
	}
}

// activeDetailContent returns the rendered content for the current view mode.
func (m Model) activeDetailContent() string {
	switch m.detailViewMode {
	case viewModeJSON:
		if m.detailJSON != "" {
			return m.detailJSON
		}
	case viewModeYAML:
		if m.detailYAML != "" {
			return m.detailYAML
		}
	case viewModeFormatted:
		// handled below
	}
	return m.detailFormatted
}

// ─── Search helpers ───────────────────────────────────────────────────────────

// rebuildSearch recomputes all match positions in the current detail content
// and re-renders the viewport with highlights applied.
func (m *Model) rebuildSearch() {
	if m.searchText == "" {
		m.searchMatches = nil
		m.searchCurrent = 0
		m.viewport.SetContent(m.detailContent)
		return
	}

	source := m.detailContent
	lines := strings.Split(source, "\n")
	lower := strings.ToLower(m.searchText)

	m.searchMatches = nil
	for lineIdx, line := range lines {
		plain := stripANSI(line)
		lplain := strings.ToLower(plain)
		pos := 0
		for {
			idx := strings.Index(lplain[pos:], lower)
			if idx < 0 {
				break
			}
			abs := pos + idx
			m.searchMatches = append(m.searchMatches, searchMatch{
				line:  lineIdx,
				start: abs,
				end:   abs + len(m.searchText),
			})
			pos = abs + len(m.searchText)
		}
	}

	if m.searchCurrent >= len(m.searchMatches) {
		m.searchCurrent = 0
	}

	m.applySearchHighlights(lines)
	if len(m.searchMatches) > 0 {
		m.scrollToMatch(m.searchCurrent)
	}
}

// applySearchHighlights injects ANSI background highlights into the content
// and pushes it into the viewport.  The source lines must match m.detailContent.
func (m *Model) applySearchHighlights(lines []string) {
	// Group matches by line number
	type lineGroup struct {
		ranges  [][2]int // {start, end} in plain-text coords
		absIdxs []int    // corresponding index into m.searchMatches
	}
	byLine := make(map[int]*lineGroup, len(m.searchMatches))
	for absIdx, sm := range m.searchMatches {
		lg := byLine[sm.line]
		if lg == nil {
			lg = &lineGroup{}
			byLine[sm.line] = lg
		}
		lg.ranges = append(lg.ranges, [2]int{sm.start, sm.end})
		lg.absIdxs = append(lg.absIdxs, absIdx)
	}

	result := make([]string, len(lines))
	for i, line := range lines {
		if lg, ok := byLine[i]; ok {
			result[i] = injectBgHighlights(line, lg.ranges, lg.absIdxs, m.searchCurrent)
		} else {
			result[i] = line
		}
	}
	m.viewport.SetContent(strings.Join(result, "\n"))
}

// scrollToMatch scrolls the viewport so the idx-th match is visible.
func (m *Model) scrollToMatch(idx int) {
	if idx >= len(m.searchMatches) {
		return
	}
	targetLine := m.searchMatches[idx].line
	// Position the match roughly 1/4 from the top of the visible area.
	offset := targetLine - m.viewport.Height/4
	if offset < 0 {
		offset = 0
	}
	m.viewport.SetYOffset(offset)
}

// nextSearchMatch advances to the next match (wrapping).
func (m *Model) nextSearchMatch() {
	if len(m.searchMatches) == 0 {
		return
	}
	m.searchCurrent = (m.searchCurrent + 1) % len(m.searchMatches)
	m.applySearchHighlights(strings.Split(m.detailContent, "\n"))
	m.scrollToMatch(m.searchCurrent)
}

// prevSearchMatch moves to the previous match (wrapping).
func (m *Model) prevSearchMatch() {
	if len(m.searchMatches) == 0 {
		return
	}
	m.searchCurrent = (m.searchCurrent - 1 + len(m.searchMatches)) % len(m.searchMatches)
	m.applySearchHighlights(strings.Split(m.detailContent, "\n"))
	m.scrollToMatch(m.searchCurrent)
}

// clearSearch closes the search bar and restores the unmodified content.
func (m *Model) clearSearch() {
	m.searching = false
	m.searchText = ""
	m.searchInput.SetValue("")
	m.searchInput.Blur()
	m.searchMatches = nil
	m.searchCurrent = 0
	m.viewport.SetContent(m.detailContent)
}

func (m Model) filteredManifests() []maestro.ResourceBundleSummary {
	if m.filterText == "" {
		return m.manifests
	}
	lower := strings.ToLower(m.filterText)
	var out []maestro.ResourceBundleSummary
	for _, mw := range m.manifests {
		if strings.Contains(strings.ToLower(mw.Name), lower) {
			out = append(out, mw)
		}
	}
	return out
}

func (m Model) selectedManifest() *maestro.ResourceBundleSummary {
	visible := m.filteredManifests()
	if len(visible) == 0 || m.manifestCursor >= len(visible) {
		return nil
	}
	v := visible[m.manifestCursor]
	return &v
}

// detailPanelDims computes width and height for the right/detail panel.
func (m Model) detailPanelDims() (int, int) {
	leftW := int(float64(m.width) * 0.40)
	rightW := m.width - leftW
	rightH := m.height - 2 // minus help bar
	return rightW, rightH
}

// ─── View ─────────────────────────────────────────────────────────────────────

// View implements tea.Model. It renders the current screen state.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	switch m.screen {
	case screenConnect:
		return m.viewConnect()
	case screenMain:
		return m.viewMain()
	}
	return ""
}

// ─── Connect screen ───────────────────────────────────────────────────────────

func (m Model) viewConnect() string {
	title := styleModalTitle.Render("Connect to Maestro")

	insecureVal := "[ ] Skip TLS"
	if m.connectInsecure {
		insecureVal = "[x] Skip TLS"
	}
	if m.connectFocusIdx == 2 {
		insecureVal = styleInputFocused.Render(insecureVal)
	} else {
		insecureVal = styleInputNormal.Render(insecureVal)
	}

	connectBtn := "  Connect  "
	if m.connectFocusIdx == 3 {
		connectBtn = styleButtonFocused.Render(connectBtn)
	} else {
		connectBtn = styleButton.Render(connectBtn)
	}

	spinner := ""
	if m.connectLoading {
		spinner = " " + spinnerFrames[m.spinnerIdx]
	}

	errLine := ""
	if m.errMsg2 != "" {
		errLine = "\n" + styleErrMsg.Render("Error: "+m.errMsg2)
	}

	labelStyle := styleDetailKey

	epLabel := labelStyle.Render("HTTP Endpoint:")
	tokLabel := labelStyle.Render("Token:        ")

	content := strings.Join([]string{
		title,
		"",
		epLabel + " " + m.connectInputs[0].View(),
		tokLabel + " " + m.connectInputs[1].View(),
		"",
		insecureVal,
		"",
		connectBtn + spinner,
		errLine,
		"",
		styleHelpDesc.Render("[Tab] next  [Enter] connect  [Ctrl+C] quit"),
	}, "\n")

	modal := styleModal.Width(60).Render(content)

	// Center the modal vertically
	topPad := (m.height - lipgloss.Height(modal)) / 2
	if topPad < 0 {
		topPad = 0
	}

	padded := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, modal)
	top := strings.Repeat("\n", topPad)
	return top + padded
}

// ─── Main screen ──────────────────────────────────────────────────────────────

func (m Model) viewMain() string {
	leftW := int(float64(m.width) * 0.40)
	rightW := m.width - leftW
	totalH := m.height - 1 // minus help bar

	consumerH := int(float64(totalH) * 0.40)
	manifestH := totalH - consumerH

	left := lipgloss.JoinVertical(lipgloss.Left,
		m.viewConsumers(leftW, consumerH),
		m.viewManifests(leftW, manifestH),
	)
	right := m.viewDetail(rightW, totalH)

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	help := m.viewHelp()

	view := lipgloss.JoinVertical(lipgloss.Left, body, help)

	// Overlay modals
	if m.showCreateConsumer {
		view = m.overlayModal(view, m.viewCreateConsumerModal())
	} else if m.showConfirm {
		view = m.overlayModal(view, m.viewConfirmModal())
	}

	return view
}

func (m Model) viewConsumers(w, h int) string {
	isFocused := m.focused == panelConsumers

	title := "Consumers"
	if isFocused {
		title = stylePanelTitleFocused.Render(title)
	} else {
		title = stylePanelTitle.Render(title)
	}

	innerW := w - 4
	innerH := h - 3
	if innerH < 1 {
		innerH = 1
	}

	var rows []string
	for i, c := range m.consumers {
		if i < m.consumerOffset || i >= m.consumerOffset+innerH {
			continue
		}
		cursor := "  "
		line := c.Name
		if i == m.consumerCursor {
			cursor = styleItemSelected.Render("> ")
			line = styleItemSelected.Render(padRight(line, innerW-2))
		} else {
			line = styleItemNormal.Render(line)
		}
		rows = append(rows, cursor+line)
	}

	if len(m.consumers) == 0 {
		rows = append(rows, styleStatusUnk.Render("  (no consumers)"))
	}

	content := strings.Join(rows, "\n")

	bs := styleBorderNormal
	if isFocused {
		bs = styleBorderFocused
	}

	return bs.Width(w - 2).Height(h - 2).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			title,
			content,
		),
	)
}

func (m Model) viewManifests(w, h int) string {
	isFocused := m.focused == panelManifests

	watchBadge := ""
	if m.watching {
		watchBadge = " " + styleWatchBadge.Render("[WATCH]")
	}
	var title string
	if isFocused {
		title = stylePanelTitleFocused.Render("ManifestWorks") + watchBadge
	} else {
		title = stylePanelTitle.Render("ManifestWorks") + watchBadge
	}

	innerW := w - 4
	innerH := h - 4
	if innerH < 1 {
		innerH = 1
	}

	// Filter row
	var filterRow string
	switch {
	case m.filtering:
		filterRow = styleFilterActive.Render("[/] ") + m.filterInput.View()
	case m.filterText != "":
		filterRow = styleFilterActive.Render("[/] filter: ") + m.filterText
	default:
		filterRow = styleHelpDesc.Render("[/] to filter")
	}

	visible := m.filteredManifests()
	var rows []string
	for i, mw := range visible {
		if i < m.manifestOffset || i >= m.manifestOffset+innerH {
			continue
		}
		applied, available := workConditions(mw.Conditions)
		icon := workStatusIcon(applied, available, len(mw.Conditions) > 0)
		name := padRight(mw.Name, innerW-5)
		cursor := "  "
		line := name + " " + icon
		if i == m.manifestCursor {
			cursor = styleItemSelected.Render("> ")
			line = styleItemSelected.Render(padRight(name+" "+icon, innerW-2))
		} else {
			line = styleItemNormal.Render(line)
		}
		rows = append(rows, cursor+line)
	}
	if len(visible) == 0 {
		rows = append(rows, styleStatusUnk.Render("  (no manifests)"))
	}

	bs := styleBorderNormal
	if isFocused {
		bs = styleBorderFocused
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		filterRow,
		strings.Join(rows, "\n"),
	)

	return bs.Width(w - 2).Height(h - 2).Render(content)
}

func (m Model) viewDetail(w, h int) string {
	isFocused := m.focused == panelDetail

	modeTag := styleJSONModeBadge.Render("[" + m.detailViewMode.String() + "]")
	var title string
	switch {
	case m.watching:
		title = stylePanelTitleWatch.Render("ManifestWork Detail") + " " + styleWatchBadge.Render("[WATCH]") + " " + modeTag
	case isFocused:
		title = stylePanelTitleFocused.Render("ManifestWork Detail") + " " + modeTag
	default:
		title = stylePanelTitle.Render("ManifestWork Detail") + " " + modeTag
	}

	spinner := ""
	if m.loading {
		spinner = " " + spinnerFrames[m.spinnerIdx]
	}

	statusLine := ""
	if m.statusMsg != "" {
		statusLine = styleStatusMsg.Render(m.statusMsg)
	}
	if m.errMsg2 != "" {
		statusLine = styleErrMsg.Render("Error: " + m.errMsg2)
	}

	// Search bar — always one row tall so viewport height stays constant.
	searchBar := m.viewSearchBar(w - 4)

	// Account for: border(2) + title(1) + status(1) + search(1) = 5 overhead rows.
	m.viewport.Width = w - 4
	m.viewport.Height = h - 6
	if m.viewport.Height < 1 {
		m.viewport.Height = 1
	}

	bs := styleBorderNormal
	if isFocused {
		bs = styleBorderFocused
	}

	inner := lipgloss.JoinVertical(lipgloss.Left,
		title+spinner,
		statusLine,
		searchBar,
		m.viewport.View(),
	)

	return bs.Width(w - 2).Height(h - 2).Render(inner)
}

// condStatusTrue is the condition status string for a satisfied condition.
const condStatusTrue = "True"

// viewSearchBar renders the one-row search bar inside the detail panel.
func (m Model) viewSearchBar(_ int) string {
	if m.searching {
		count := ""
		if len(m.searchMatches) == 0 && m.searchText != "" {
			count = styleSearchNoMatch.Render(" (no matches)")
		} else if len(m.searchMatches) > 0 {
			count = styleSearchCount.Render(
				fmt.Sprintf(" %d/%d", m.searchCurrent+1, len(m.searchMatches)),
			)
		}
		return styleSearchBar.Render(m.searchInput.View()) + count
	}
	if m.searchText != "" {
		// Search closed but still highlighting — show match count + nav hint.
		count := styleSearchCount.Render(
			fmt.Sprintf("%d/%d", m.searchCurrent+1, len(m.searchMatches)),
		)
		return styleSearchBar.Render("/ "+m.searchText) + " " + count +
			"  " + styleHelpDesc.Render("[n] next  [N] prev  [/] reopen  [Esc] clear")
	}
	return styleHelpDesc.Render("[/] search")
}

func (m Model) viewHelp() string {
	var parts []string
	addKey := func(key, desc string) {
		parts = append(parts, styleHelpKey.Render(key)+" "+styleHelpDesc.Render(desc))
	}

	addKey("[Tab]", "panel")
	switch m.focused {
	case panelConsumers:
		addKey("[n]", "new")
		addKey("[d]", "del")
		addKey("[y]", "copy")
		addKey("[r]", "refresh")
		addKey("[↑↓]", "nav")
		addKey("[Enter]", "select")
	case panelManifests:
		addKey("[/]", "filter")
		addKey("[w]", "watch")
		addKey("[v]", "view mode")
		addKey("[y]", "copy")
		addKey("[d]", "del")
		addKey("[r]", "refresh")
		addKey("[↑↓]", "nav")
	case panelDetail:
		addKey("[w]", "watch")
		addKey("[v]", "view mode")
		addKey("[y]", "copy")
		addKey("[r]", "refresh")
		addKey("[↑↓/PgUp/PgDn]", "scroll")
	}
	addKey("[Ctrl+C]", "quit")

	return styleHelpDesc.Render(" " + strings.Join(parts, "  "))
}

// ─── Modals ───────────────────────────────────────────────────────────────────

func (m Model) viewCreateConsumerModal() string {
	title := styleModalTitle.Render("Create Consumer")
	content := strings.Join([]string{
		title,
		"",
		styleDetailKey.Render("Name: ") + m.createInput.View(),
		"",
		styleHelpDesc.Render("[Enter] create  [Esc] cancel"),
	}, "\n")
	return styleModal.Width(50).Render(content)
}

func (m Model) viewConfirmModal() string {
	title := styleModalTitle.Render("Confirm Delete")
	content := strings.Join([]string{
		title,
		"",
		styleDetailValue.Render(m.confirmMsg),
		"",
		styleHelpDesc.Render("[y] confirm  [Esc] cancel"),
	}, "\n")
	return styleModal.Width(50).Render(content)
}

func (m Model) overlayModal(_ string, modal string) string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("#1F2937")),
	)
}

// ─── Detail rendering ─────────────────────────────────────────────────────────

func renderDetail(d *maestro.ManifestWorkDetails) string {
	if d == nil {
		return styleStatusUnk.Render("(no detail available)")
	}

	var sb strings.Builder

	kv := func(key, val string) string {
		return styleDetailKey.Render(padRight(key, 12)) + " " + styleDetailValue.Render(val)
	}

	sb.WriteString(kv("Name:", d.Name) + "\n")
	sb.WriteString(kv("Consumer:", d.ConsumerName) + "\n")
	sb.WriteString(kv("Version:", fmt.Sprintf("%d", d.Version)) + "\n")
	sb.WriteString(kv("Created:", d.CreatedAt) + "\n")
	sb.WriteString(kv("Updated:", d.UpdatedAt) + "\n")

	sb.WriteString("\n")
	sb.WriteString(styleDetailHeader.Render("Conditions:") + "\n")
	if len(d.Conditions) == 0 {
		sb.WriteString("  " + styleStatusUnk.Render("(none)") + "\n")
	} else {
		for _, c := range d.Conditions {
			icon := conditionIcon(c.Status)
			sb.WriteString(fmt.Sprintf("  %s %s", icon, styleDetailValue.Render(c.Type)) + "\n")
			if c.Message != "" {
				sb.WriteString("    " + styleHelpDesc.Render(c.Message) + "\n")
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString(styleDetailHeader.Render(fmt.Sprintf("Manifests (%d):", len(d.Manifests))) + "\n")
	for _, mf := range d.Manifests {
		ns := mf.Namespace
		if ns == "" {
			ns = "(cluster)"
		}
		sb.WriteString(fmt.Sprintf("  • %s/%s (%s)\n",
			styleDetailValue.Render(mf.Kind),
			styleDetailValue.Render(mf.Name),
			styleHelpDesc.Render(ns),
		))
	}

	if len(d.ResourceStatus) > 0 {
		sb.WriteString("\n")
		sb.WriteString(styleDetailHeader.Render("Resource Status:") + "\n")
		for _, rs := range d.ResourceStatus {
			kind := rs.Kind
			if kind == "" {
				kind = "Unknown"
			}
			header := fmt.Sprintf("  %s/%s:", kind, rs.Name)
			sb.WriteString(styleDetailKey.Render(header) + "\n")
			for _, c := range rs.Conditions {
				icon := conditionIcon(c.Status)
				sb.WriteString(fmt.Sprintf("    %s %s\n", icon, styleDetailValue.Render(c.Type)))
			}
		}
	}

	return sb.String()
}

// ─── Utility functions ────────────────────────────────────────────────────────

func padRight(s string, n int) string {
	vis := lipgloss.Width(s)
	if vis >= n {
		return s
	}
	return s + strings.Repeat(" ", n-vis)
}

func workConditions(conds []maestro.ConditionSummary) (applied, available bool) {
	for _, c := range conds {
		if c.Type == "Applied" && c.Status == condStatusTrue {
			applied = true
		}
		if c.Type == "Available" && c.Status == condStatusTrue {
			available = true
		}
	}
	return
}
