package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"text/tabwriter"

	regel "github.com/sodiqscript111/Regel"
	"github.com/sodiqscript111/Regel/patterns"
)

const (
	exitPassed = 0
	exitFailed = 1
	exitError  = 2

	demoPattern = "CheckThenAct"
)

const usage = `Regel writes TLA+ models from Go and checks them with TLC.

Usage:
  regel list                         List the patterns
  regel show <pattern> [--fixed]     Print the generated TLA+
  regel check <pattern> [--fixed]    Run TLC and print the result
  regel demo                         Run a broken pattern, then its fix
  regel doctor                       Report whether Java and tla2tools.jar were found

Exit codes: 0 for a pass, 1 for a failed check, 2 for a usage or tool error.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

type cli struct {
	stdout io.Writer
	stderr io.Writer
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	c := cli{stdout: stdout, stderr: stderr}
	if len(args) == 0 {
		return c.usageError("no command given")
	}
	command, rest := args[0], args[1:]
	switch command {
	case "list":
		return c.withNoArguments(command, rest, c.list)
	case "show":
		return c.withPattern(command, rest, c.show)
	case "check":
		return c.withPattern(command, rest, func(spec *regel.Spec) int { return c.check(ctx, spec) })
	case "demo":
		return c.withNoArguments(command, rest, func() int { return c.demo(ctx) })
	case "doctor":
		return c.withNoArguments(command, rest, c.doctor)
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return exitPassed
	}
	return c.usageError(fmt.Sprintf("unknown command %q", command))
}

func (c cli) usageError(problem string) int {
	fmt.Fprintf(c.stderr, "regel: %s\n\n%s", problem, usage)
	return exitError
}

func (c cli) withNoArguments(command string, args []string, action func() int) int {
	if len(args) > 0 {
		return c.usageError(fmt.Sprintf("%s takes no arguments", command))
	}
	return action()
}

func (c cli) withPattern(command string, args []string, action func(*regel.Spec) int) int {
	name, fixed, err := parsePatternArguments(command, args)
	if err != nil {
		return c.usageError(err.Error())
	}
	pattern, found := patterns.Find(name)
	if !found {
		return c.usageError(fmt.Sprintf("unknown pattern %q: run regel list", name))
	}
	broken, repaired := pattern.Build()
	if fixed {
		return action(repaired)
	}
	return action(broken)
}

func parsePatternArguments(command string, args []string) (string, bool, error) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	fixed := flags.Bool("fixed", false, "use the fixed version")
	var names []string
	for {
		if err := flags.Parse(args); err != nil {
			return "", false, fmt.Errorf("%s: %w", command, err)
		}
		if flags.NArg() == 0 {
			break
		}
		names = append(names, flags.Arg(0))
		args = flags.Args()[1:]
	}
	if len(names) != 1 {
		return "", false, errors.New(command + " takes one pattern name")
	}
	return names[0], *fixed, nil
}

func (c cli) list() int {
	table := tabwriter.NewWriter(c.stdout, 0, 0, 2, ' ', 0)
	for _, pattern := range patterns.All() {
		fmt.Fprintf(table, "%s\t%s\n", pattern.Name, pattern.Summary)
	}
	table.Flush()
	return exitPassed
}

func (c cli) show(spec *regel.Spec) int {
	source, err := spec.Source()
	if err != nil {
		fmt.Fprintf(c.stderr, "regel: %v\n", err)
		return exitError
	}
	fmt.Fprint(c.stdout, source)
	return exitPassed
}

func (c cli) check(ctx context.Context, spec *regel.Spec) int {
	result, err := spec.Check(ctx)
	if err != nil {
		fmt.Fprintf(c.stderr, "regel: %v\n", err)
		return exitError
	}
	fmt.Fprintln(c.stdout, result)
	return exitCode(result)
}

func exitCode(result *regel.Result) int {
	switch {
	case result.Passed():
		return exitPassed
	case result == nil, result.Verdict == regel.ToolError:
		return exitError
	}
	return exitFailed
}

func (c cli) demo(ctx context.Context) int {
	pattern, found := patterns.Find(demoPattern)
	if !found {
		return c.usageError(fmt.Sprintf("unknown pattern %q", demoPattern))
	}
	broken, fixed := pattern.Build()
	fmt.Fprintf(c.stdout, "%s: %s\n\nThe broken version:\n\n", pattern.Name, pattern.Summary)
	brokenExit := c.check(ctx, broken)
	if brokenExit == exitError {
		return exitError
	}
	fmt.Fprintf(c.stdout, "\nThe fixed version:\n\n")
	fixedExit := c.check(ctx, fixed)
	if fixedExit == exitError {
		return exitError
	}
	if brokenExit != exitFailed || fixedExit != exitPassed {
		fmt.Fprintln(c.stderr, "regel: the demo did not behave as expected")
		return exitFailed
	}
	return exitPassed
}

func (c cli) doctor() int {
	java, javaErr := regel.FindJava()
	jar, jarErr := regel.FindJar()
	c.report("Java", java, javaErr)
	c.report("tla2tools.jar", jar, jarErr)
	if javaErr != nil || jarErr != nil {
		fmt.Fprintln(c.stdout, "Regel needs both to run TLC. Set REGEL_JAVA and REGEL_TLA2TOOLS to their paths.")
		return exitError
	}
	return exitPassed
}

func (c cli) report(tool, path string, err error) {
	if err != nil {
		fmt.Fprintf(c.stdout, "%s: NOT FOUND (%v)\n", tool, err)
		return
	}
	fmt.Fprintf(c.stdout, "%s: found at %s\n", tool, path)
}
