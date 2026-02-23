// Package tui provides a terminal UI for browsing Maestro consumers and ManifestWorks.
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	colorPrimary   = lipgloss.Color("#7C3AED") // purple
	colorSecondary = lipgloss.Color("#06B6D4") // cyan
	colorSuccess   = lipgloss.Color("#10B981") // green
	colorWarning   = lipgloss.Color("#F59E0B") // amber
	colorError     = lipgloss.Color("#EF4444") // red
	colorMuted     = lipgloss.Color("#6B7280") // gray
	colorFocused   = lipgloss.Color("#3B82F6") // blue
	colorSelected  = lipgloss.Color("#1E40AF") // dark blue

	// Panel border styles
	styleBorderNormal = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorMuted)

	styleBorderFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorFocused)

	// Panel title styles
	stylePanelTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSecondary)

	stylePanelTitleFocused = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorFocused)

	stylePanelTitleWatch = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWarning)

	// List item styles
	styleItemNormal = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB"))

	styleItemSelected = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(colorSelected)

	// Status indicator styles
	styleStatusOK  = lipgloss.NewStyle().Foreground(colorSuccess)
	styleStatusErr = lipgloss.NewStyle().Foreground(colorError)
	styleStatusUnk = lipgloss.NewStyle().Foreground(colorMuted)

	// Condition badge styles
	styleCondTrue  = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	styleCondFalse = lipgloss.NewStyle().Foreground(colorError).Bold(true)
	styleCondUnk   = lipgloss.NewStyle().Foreground(colorMuted)

	// Detail section styles
	styleDetailKey = lipgloss.NewStyle().
			Foreground(colorMuted).
			Bold(true)

	styleDetailValue = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5E7EB"))

	styleDetailHeader = lipgloss.NewStyle().
				Foreground(colorSecondary).
				Bold(true).
				Underline(true)

	// Help bar
	styleHelpKey = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	styleHelpDesc = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Status bar
	styleStatusMsg = lipgloss.NewStyle().
			Foreground(colorSuccess)

	styleErrMsg = lipgloss.NewStyle().
			Foreground(colorError)

	// Modal styles
	styleModal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2)

	styleModalTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	styleInputFocused = lipgloss.NewStyle().
				Foreground(colorFocused)

	styleInputNormal = lipgloss.NewStyle().
				Foreground(colorMuted)

	styleButton = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorPrimary).
			Padding(0, 2)

	styleButtonFocused = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(colorFocused).
				Padding(0, 2)

	// Watch indicator
	styleWatchBadge = lipgloss.NewStyle().
			Foreground(colorWarning).
			Bold(true)

	// Filter indicator
	styleFilterActive = lipgloss.NewStyle().
				Foreground(colorWarning)

	// View mode badge (shown in detail panel title)
	styleJSONModeBadge = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#94A3B8")).
				Bold(false)

	// ── Syntax-highlighting styles ────────────────────────────────────────────

	styleJSONKey    = lipgloss.NewStyle().Foreground(lipgloss.Color("#7DD3FC")) // sky blue  — keys
	styleJSONString = lipgloss.NewStyle().Foreground(lipgloss.Color("#86EFAC")) // green     — strings
	styleJSONNumber = lipgloss.NewStyle().Foreground(lipgloss.Color("#FDE68A")) // amber     — numbers
	styleJSONBool   = lipgloss.NewStyle().Foreground(lipgloss.Color("#C4B5FD")) // lavender  — true/false
	styleJSONNull   = lipgloss.NewStyle().Foreground(colorMuted)                //            — null/~
	styleJSONPunct  = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")) // slate     — punctuation

	// ── Search bar styles ─────────────────────────────────────────────────────

	styleSearchBar     = lipgloss.NewStyle().Foreground(colorFocused)
	styleSearchCount   = lipgloss.NewStyle().Foreground(colorMuted)
	styleSearchNoMatch = lipgloss.NewStyle().Foreground(colorError)
)

// ─── Condition / status icons ─────────────────────────────────────────────────

// conditionIcon returns a colored icon for a condition status
func conditionIcon(status string) string {
	switch status {
	case condStatusTrue:
		return styleCondTrue.Render("✓")
	case "False":
		return styleCondFalse.Render("✗")
	default:
		return styleCondUnk.Render("?")
	}
}

// workStatusIcon returns a status icon for a ManifestWork based on its conditions
func workStatusIcon(applied, available bool, hasConditions bool) string {
	if !hasConditions {
		return styleStatusUnk.Render("?")
	}
	if applied && available {
		return styleStatusOK.Render("✓")
	}
	return styleStatusErr.Render("✗")
}

// ─── JSON syntax colorizer ────────────────────────────────────────────────────

// colorizeJSON applies terminal colors to a pretty-printed JSON string.
func colorizeJSON(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, colorizeJSONLine(line))
	}
	return strings.Join(out, "\n")
}

func colorizeJSONLine(line string) string {
	if line == "" {
		return ""
	}
	// Preserve leading whitespace
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]

	// Key-value line: starts with a quoted string followed by ":"
	if len(trimmed) >= 2 && trimmed[0] == '"' {
		keyEnd := findClosingQuote(trimmed, 1)
		if keyEnd > 0 && keyEnd+1 < len(trimmed) {
			afterKey := strings.TrimLeft(trimmed[keyEnd+1:], " ")
			if strings.HasPrefix(afterKey, ":") {
				key := trimmed[:keyEnd+1]
				rest := afterKey[1:] // everything after ":"
				return indent + styleJSONKey.Render(key) + styleJSONPunct.Render(":") + colorizeJSONValue(rest)
			}
		}
	}

	// Standalone value or structural token
	return indent + colorizeJSONValue(trimmed)
}

// colorizeJSONValue colorizes the value portion of a JSON line (may have leading space and trailing comma).
func colorizeJSONValue(s string) string {
	if s == "" {
		return ""
	}
	// Preserve single leading space (after ":")
	prefix := ""
	if s[0] == ' ' {
		prefix = " "
		s = s[1:]
	}
	if s == "" {
		return prefix
	}

	// Separate trailing comma
	suffix := ""
	if s[len(s)-1] == ',' {
		suffix = styleJSONPunct.Render(",")
		s = s[:len(s)-1]
	}

	var colored string
	switch {
	case s == "true" || s == "false":
		colored = styleJSONBool.Render(s)
	case s == "null":
		colored = styleJSONNull.Render(s)
	case len(s) > 0 && s[0] == '"':
		colored = styleJSONString.Render(s)
	case s == "{" || s == "}" || s == "[" || s == "]" ||
		s == "{}" || s == "[]" || s == "}," || s == "]," ||
		strings.HasPrefix(s, "}") || strings.HasPrefix(s, "]"):
		colored = styleJSONPunct.Render(s)
	case len(s) > 0 && (s[0] == '-' || (s[0] >= '0' && s[0] <= '9')):
		colored = styleJSONNumber.Render(s)
	default:
		colored = s
	}
	return prefix + colored + suffix
}

// findClosingQuote returns the index of the closing unescaped '"' in s, starting from pos.
func findClosingQuote(s string, pos int) int {
	for i := pos; i < len(s); i++ {
		if s[i] == '\\' {
			i++ // skip escaped char
			continue
		}
		if s[i] == '"' {
			return i
		}
	}
	return -1
}

// ─── YAML syntax colorizer ────────────────────────────────────────────────────

// colorizeYAML applies terminal colors to a YAML string.
func colorizeYAML(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, colorizeYAMLLine(line))
	}
	return strings.Join(out, "\n")
}

func colorizeYAMLLine(line string) string {
	if line == "" {
		return ""
	}
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]

	// Document separator
	if trimmed == "---" || trimmed == "..." {
		return styleJSONPunct.Render(line)
	}

	// List item prefix
	listPrefix := ""
	rest := trimmed
	if strings.HasPrefix(trimmed, "- ") {
		listPrefix = styleJSONPunct.Render("- ")
		rest = trimmed[2:]
	} else if trimmed == "-" {
		return indent + styleJSONPunct.Render("-")
	}

	// Key: value  or  key:  (nested mapping)
	colonIdx := strings.Index(rest, ": ")
	if colonIdx > 0 {
		key := rest[:colonIdx]
		val := rest[colonIdx+2:]
		return indent + listPrefix + styleJSONKey.Render(key) + styleJSONPunct.Render(": ") + colorizeYAMLValue(val)
	}
	if strings.HasSuffix(rest, ":") && !strings.Contains(rest[:len(rest)-1], ":") {
		key := rest[:len(rest)-1]
		return indent + listPrefix + styleJSONKey.Render(key) + styleJSONPunct.Render(":")
	}

	// Pure value (array scalar, etc.)
	return indent + listPrefix + colorizeYAMLValue(rest)
}

// colorizeYAMLValue colorizes a YAML scalar value.
func colorizeYAMLValue(s string) string {
	if s == "" {
		return ""
	}
	switch s {
	case "true", "false", "True", "False", "TRUE", "FALSE", "yes", "no":
		return styleJSONBool.Render(s)
	case "null", "~", "Null", "NULL":
		return styleJSONNull.Render(s)
	}
	// Quoted string
	if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
		return styleJSONString.Render(s)
	}
	// Number
	if len(s) > 0 && (s[0] == '-' || (s[0] >= '0' && s[0] <= '9')) {
		return styleJSONNumber.Render(s)
	}
	// Plain string value
	return styleJSONString.Render(s)
}

// ─── Search highlight injection ───────────────────────────────────────────────

// buildCharMap returns a slice that maps plain-text byte indices to their
// corresponding byte offsets inside the ANSI-colored string ansiStr.
// charMap[i] is the byte offset in ansiStr where the i-th plain-text byte begins.
// A sentinel entry equal to len(ansiStr) is appended so that charMap[len(plain)]
// gives the position just past the last character.
func buildCharMap(ansiStr string) []int {
	var charMap []int
	i := 0
	for i < len(ansiStr) {
		// Skip an ANSI CSI escape sequence: ESC '[' <params> <letter>
		if ansiStr[i] == '\x1b' && i+1 < len(ansiStr) && ansiStr[i+1] == '[' {
			j := i + 2
			for j < len(ansiStr) && (ansiStr[j] == ';' || (ansiStr[j] >= '0' && ansiStr[j] <= '9')) {
				j++
			}
			if j < len(ansiStr) {
				j++ // consume the final command letter
			}
			i = j
			continue
		}
		// Regular byte — record its position in the ANSI string.
		charMap = append(charMap, i)
		i++
	}
	charMap = append(charMap, i) // sentinel: points just past end of ansiStr
	return charMap
}

// injectBgHighlights injects ANSI background-colour codes into coloredLine so
// that each range in ranges is visually highlighted.  Ranges are expressed as
// [start, end) byte offsets into the *plain* (ANSI-stripped) version of the
// line.  absIdxs[k] is the index of ranges[k] in the global searchMatches
// slice; currentIdx is the currently selected match index.  The current match
// is highlighted green (\x1b[42m); all others are amber (\x1b[43m).
func injectBgHighlights(coloredLine string, ranges [][2]int, absIdxs []int, currentIdx int) string {
	if len(ranges) == 0 {
		return coloredLine
	}

	charMap := buildCharMap(coloredLine)

	var sb strings.Builder
	prev := 0 // last written byte position in coloredLine

	for k, r := range ranges {
		start, end := r[0], r[1]

		// Clamp to the valid charMap range.
		if start >= len(charMap)-1 {
			continue
		}
		if end >= len(charMap) {
			end = len(charMap) - 1
		}

		byteStart := charMap[start]
		byteEnd := charMap[end]

		if byteStart < prev {
			continue // overlapping range — skip
		}

		// Emit text before this match (preserves existing ANSI codes).
		sb.WriteString(coloredLine[prev:byteStart])

		// Inject background colour.
		if absIdxs[k] == currentIdx {
			sb.WriteString("\x1b[42m") // green — current match
		} else {
			sb.WriteString("\x1b[43m") // amber — other matches
		}

		// Emit the matched text (keeps its foreground syntax colour).
		sb.WriteString(coloredLine[byteStart:byteEnd])

		// Reset background only (foreground remains unchanged).
		sb.WriteString("\x1b[49m")

		prev = byteEnd
	}

	// Emit any remaining text after the last match.
	sb.WriteString(coloredLine[prev:])
	return sb.String()
}
