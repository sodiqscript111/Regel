package regel

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
)

const (
	codeInvariantViolated        = 2110
	codeInvariantViolatedAtStart = 2107
	codeBehaviorHeader           = 2121
	codeBackToState              = 2122
	codeStats                    = 2199
	codeState                    = 2217
	codeStuttering               = 2218

	severityError = 1

	startAction   = "start"
	pcVariable    = "pc"
	stateBullet   = `/\ `
	columnPadding = 2
)

var (
	markerPattern  = regexp.MustCompile(`@!@!@(?:STARTMSG (\d+):(\d+)|ENDMSG \d+) @!@!@`)
	stateHeader    = regexp.MustCompile(`^(\d+): <([^\s(>]+)`)
	backToState    = regexp.MustCompile(`^(\d+): Back to state`)
	pcEntryPattern = regexp.MustCompile(`(\w+) :> "(\w+)"`)
)

type message struct {
	code     int
	severity int
	text     string
}

func parseMessages(output string) ([]*message, string) {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	var messages, open []*message
	var loose strings.Builder
	write := func(text string) {
		if len(open) == 0 {
			loose.WriteString(text)
		}
		for _, m := range open {
			m.text += text
		}
	}
	position := 0
	for _, marker := range markerPattern.FindAllStringSubmatchIndex(output, -1) {
		write(output[position:marker[0]])
		position = marker[1]
		if marker[2] < 0 {
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
			continue
		}
		code, _ := strconv.Atoi(output[marker[2]:marker[3]])
		severity, _ := strconv.Atoi(output[marker[4]:marker[5]])
		m := &message{code: code, severity: severity}
		messages = append(messages, m)
		open = append(open, m)
	}
	write(output[position:])
	return messages, loose.String()
}

type State struct {
	Number int
	Action string
	Worker string
	Vars   map[string]string
}

type Trace []State

func parseState(text string) (State, bool) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	header := stateHeader.FindStringSubmatch(lines[0])
	if header == nil {
		return State{}, false
	}
	number, _ := strconv.Atoi(header[1])
	action := strings.TrimPrefix(header[2], stepPrefix)
	if number == 1 {
		action = startAction
	}
	return State{Number: number, Action: action, Vars: parseVars(lines[1:])}, true
}

func parseVars(lines []string) map[string]string {
	vars := map[string]string{}
	current := ""
	for _, line := range lines {
		name, value, found := strings.Cut(strings.TrimPrefix(line, stateBullet), " = ")
		if found && (current == "" || strings.HasPrefix(line, stateBullet)) {
			current = name
			vars[current] = strings.TrimSpace(value)
			continue
		}
		if current != "" && strings.TrimSpace(line) != "" {
			vars[current] += " " + strings.TrimSpace(line)
		}
	}
	return vars
}

type pcEntry struct {
	worker string
	label  string
}

func (s State) positions() []pcEntry {
	var entries []pcEntry
	for _, match := range pcEntryPattern.FindAllStringSubmatch(s.Vars[pcVariable], -1) {
		entries = append(entries, pcEntry{worker: match[1], label: match[2]})
	}
	return entries
}

func labelOf(entries []pcEntry, worker string) string {
	for _, entry := range entries {
		if entry.worker == worker {
			return entry.label
		}
	}
	return ""
}

func movedWorker(before, after State) string {
	positions := before.positions()
	following := after.positions()
	var atLabel []string
	for _, entry := range positions {
		if labelOf(following, entry.worker) != entry.label {
			return entry.worker
		}
		if entry.label == after.Action {
			atLabel = append(atLabel, entry.worker)
		}
	}
	if len(atLabel) == 1 {
		return atLabel[0]
	}
	return ""
}

func (r *Result) readTrace(messages []*message) {
	for _, m := range messages {
		switch m.code {
		case codeState:
			if state, ok := parseState(m.text); ok {
				r.Trace = append(r.Trace, state)
			}
		case codeInvariantViolatedAtStart:
			_, rest, _ := strings.Cut(m.text, "\n")
			r.Trace = append(r.Trace, State{Number: 1, Action: startAction, Vars: parseVars(strings.Split(rest, "\n"))})
		case codeStuttering:
			r.Stutters = true
		case codeBackToState:
			if match := backToState.FindStringSubmatch(strings.TrimSpace(m.text)); match != nil {
				r.LoopsTo, _ = strconv.Atoi(match[1])
			}
		}
	}
	for i := 1; i < len(r.Trace); i++ {
		r.Trace[i].Worker = movedWorker(r.Trace[i-1], r.Trace[i])
	}
}

func (s State) description() string {
	if s.Worker == "" {
		return s.Action
	}
	return s.Worker + ": " + s.Action
}

func (s State) cells() []string {
	cells := []string{strconv.Itoa(s.Number), s.description()}
	names := make([]string, 0, len(s.Vars))
	for name := range s.Vars {
		if name != pcVariable {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	for _, name := range names {
		cells = append(cells, fmt.Sprintf("%s=%s", name, s.Vars[name]))
	}
	for _, entry := range s.positions() {
		cells = append(cells, fmt.Sprintf("%s=%s", entry.worker, entry.label))
	}
	return cells
}

func (t Trace) String() string {
	var out strings.Builder
	table := tabwriter.NewWriter(&out, 0, 0, columnPadding, ' ', 0)
	for _, state := range t {
		fmt.Fprintln(table, strings.Join(state.cells(), "\t"))
	}
	table.Flush()
	return strings.TrimRight(out.String(), "\n")
}
