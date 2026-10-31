package regel

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	exitPassed    = 0
	exitDeadlock  = 11
	exitInvariant = 12
	exitProperty  = 13
)

type Verdict string

const (
	Passed            Verdict = "passed"
	InvariantViolated Verdict = "invariant violated"
	PropertyViolated  Verdict = "property violated"
	Deadlock          Verdict = "deadlock"
	ToolError         Verdict = "tool error"
)

type Result struct {
	Verdict         Verdict
	ExitCode        int
	StatesGenerated int64
	DistinctStates  int64
	Violated        string
	Trace           Trace
	LoopsTo         int
	Stutters        bool
	RawOutput       string

	problem string
}

var (
	statsPattern     = regexp.MustCompile(`^([\d,]+) states generated, ([\d,]+) distinct states found`)
	invariantPattern = regexp.MustCompile(`^Invariant (\S+) is violated`)
)

func newResult(exitCode int, output string) *Result {
	messages, loose := parseMessages(output)
	result := &Result{
		Verdict:   verdictFor(exitCode, output),
		ExitCode:  exitCode,
		RawOutput: output,
		problem:   problemText(messages, loose),
	}
	result.readStats(messages)
	result.readViolated(messages)
	result.readTrace(messages)
	return result
}

func (r *Result) Passed() bool {
	return r != nil && r.Verdict == Passed
}

func verdictFor(exitCode int, output string) Verdict {
	switch exitCode {
	case exitPassed:
		if strings.Contains(output, "No error has been found") {
			return Passed
		}
	case exitDeadlock:
		return Deadlock
	case exitInvariant:
		return InvariantViolated
	case exitProperty:
		return PropertyViolated
	}
	return ToolError
}

func (r *Result) readStats(messages []*message) {
	for _, m := range messages {
		if m.code != codeStats {
			continue
		}
		if match := statsPattern.FindStringSubmatch(strings.TrimSpace(m.text)); match != nil {
			r.StatesGenerated = parseCount(match[1])
			r.DistinctStates = parseCount(match[2])
		}
	}
}

func parseCount(text string) int64 {
	count, err := strconv.ParseInt(strings.ReplaceAll(text, ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	return count
}

func (r *Result) readViolated(messages []*message) {
	if r.Verdict == Deadlock {
		r.Violated = NoDeadlock
	}
	for _, m := range messages {
		if m.code != codeInvariantViolated && m.code != codeInvariantViolatedAtStart {
			continue
		}
		if match := invariantPattern.FindStringSubmatch(strings.TrimSpace(m.text)); match != nil {
			r.Violated = match[1]
		}
	}
}

func problemText(messages []*message, loose string) string {
	var lines []string
	for _, line := range strings.Split(loose, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "Parsing file") && !strings.HasPrefix(line, "Semantic processing") {
			lines = append(lines, line)
		}
	}
	for _, m := range messages {
		if m.severity == severityError && m.code != codeBehaviorHeader {
			lines = append(lines, strings.TrimSpace(m.text))
		}
	}
	return strings.Join(lines, "\n")
}

func (r *Result) String() string {
	if r == nil {
		return "no result"
	}
	sections := []string{r.headline()}
	if r.Verdict == ToolError && r.problem != "" {
		sections = append(sections, r.problem)
	}
	if len(r.Trace) > 0 {
		sections = append(sections, r.Trace.String()+r.ending())
	}
	return strings.Join(sections, "\n\n")
}

func (r *Result) headline() string {
	switch r.Verdict {
	case Passed:
		return fmt.Sprintf("PASSED: %d states generated, %d distinct", r.StatesGenerated, r.DistinctStates)
	case InvariantViolated, PropertyViolated:
		return fmt.Sprintf("FAILED: %s was broken after %s", firstSet(r.Violated, "a rule"), steps(len(r.Trace)))
	case Deadlock:
		return fmt.Sprintf("FAILED: deadlock after %s, the workers are stuck", steps(len(r.Trace)))
	}
	return fmt.Sprintf("ERROR: TLC could not check this spec (exit code %d)", r.ExitCode)
}

func steps(count int) string {
	if count == 1 {
		return "1 step"
	}
	return fmt.Sprintf("%d steps", count)
}

func (r *Result) ending() string {
	switch {
	case r.LoopsTo > 0:
		return fmt.Sprintf("\n\nThen it goes back to step %d and repeats forever.", r.LoopsTo)
	case r.Stutters:
		return "\n\nThen nothing happens, forever."
	}
	return ""
}
