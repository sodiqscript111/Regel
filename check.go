package regel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	stopGracePeriod = 5 * time.Second
	removeAttempts  = 10
	removeBackoff   = 100 * time.Millisecond
)

type checkConfig struct {
	java string
	jar  string
}

type CheckOption func(*checkConfig)

func WithJava(path string) CheckOption {
	return func(config *checkConfig) {
		config.java = path
	}
}

func WithJar(path string) CheckOption {
	return func(config *checkConfig) {
		config.jar = path
	}
}

func (s *Spec) Check(ctx context.Context, options ...CheckOption) (*Result, error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	source, err := s.Source()
	if err != nil {
		return nil, err
	}
	config, err := s.Config()
	if err != nil {
		return nil, err
	}
	tools, err := findTools(options)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "regel-")
	if err != nil {
		return nil, fmt.Errorf("could not create a temp folder: %w", err)
	}
	defer removeFolder(dir)
	if err := writeModel(dir, s.name, source, config); err != nil {
		return nil, err
	}
	output, exitCode, err := tools.run(ctx, dir, s.name)
	if err != nil {
		return nil, err
	}
	return newResult(exitCode, output), nil
}

type tools struct {
	java string
	jar  string
}

func findTools(options []CheckOption) (tools, error) {
	config := checkConfig{}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	java, err := findJava(firstSet(config.java, os.Getenv("REGEL_JAVA"), "java"))
	if err != nil {
		return tools{}, err
	}
	jar, err := findJar(firstSet(config.jar, os.Getenv("REGEL_TLA2TOOLS")))
	if err != nil {
		return tools{}, err
	}
	return tools{java: java, jar: jar}, nil
}

func firstSet(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func findJava(java string) (string, error) {
	path, err := exec.LookPath(java)
	if err != nil {
		return "", fmt.Errorf("java not found at %q: pass WithJava, set REGEL_JAVA, or add java to PATH: %w", java, err)
	}
	return filepath.Abs(path)
}

func findJar(jar string) (string, error) {
	if jar == "" {
		return "", errors.New("tla2tools.jar not found: pass WithJar or set REGEL_TLA2TOOLS to its path")
	}
	info, err := os.Stat(jar)
	if err != nil {
		return "", fmt.Errorf("tla2tools.jar not found at %q: pass WithJar or set REGEL_TLA2TOOLS to its path: %w", jar, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("tla2tools.jar not found at %q: that path is a folder", jar)
	}
	return filepath.Abs(jar)
}

func writeModel(dir, name, source, config string) error {
	files := map[string]string{name + ".tla": source, name + ".cfg": config}
	for file, content := range files {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o600); err != nil {
			return fmt.Errorf("could not write %s: %w", file, err)
		}
	}
	return nil
}

func removeFolder(dir string) {
	for range removeAttempts {
		if os.RemoveAll(dir) == nil {
			return
		}
		time.Sleep(removeBackoff)
	}
}

func (t tools) arguments(dir, name string) []string {
	return []string{
		"-XX:+UseParallelGC",
		"-XX:-UsePerfData",
		"-Djava.io.tmpdir=" + dir,
		"-cp", t.jar,
		"tlc2.TLC",
		"-tool",
		"-workers", "1",
		"-metadir", filepath.Join(dir, "states"),
		"-config", name + ".cfg",
		name,
	}
}

func (t tools) run(ctx context.Context, dir, name string) (string, int, error) {
	var output bytes.Buffer
	command := exec.CommandContext(ctx, t.java, t.arguments(dir, name)...)
	command.Dir = dir
	command.Stdout = &output
	command.Stderr = &output
	command.WaitDelay = stopGracePeriod
	err := command.Run()
	if ctx.Err() != nil {
		return "", 0, fmt.Errorf("TLC was stopped: %w", ctx.Err())
	}
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		return "", 0, fmt.Errorf("could not run TLC: %w", err)
	}
	return output.String(), command.ProcessState.ExitCode(), nil
}
