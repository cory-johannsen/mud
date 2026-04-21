package e2e_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"text/template"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Package layout note: all scenario files (scenarios_*_test.go) and this suite file are
// in the same flat package (internal/e2e, package e2e_test) rather than a scenarios/
// subdirectory. This is intentional: Go does not support a TestMain in a parent package
// that governs tests in a sub-package. All test files that need to share a single TestMain
// (for lifecycle management of PostgreSQL, gameserver, and frontend subprocesses) must
// reside in the same package.

// e2eState holds all subprocess handles and addresses for the test suite.
var e2eState struct {
	HeadlessAddr string // "127.0.0.1:<port>"
}

var timingMu sync.Mutex
var timingResults []timingEntry

type timingEntry struct {
	name    string
	elapsed time.Duration
	passed  bool
}

// recordTiming registers elapsed time and pass/fail for a test (REQ-ITS-10).
// Call at the start of each scenario: defer recordTiming(t, time.Now()).
//
// Known limitation: t.Failed() is evaluated at defer time (when the test function
// returns). If a t.Cleanup callback registered after recordTiming's defer fails the
// test, the timing entry will report PASS incorrectly because t.Failed() is checked
// before t.Cleanup callbacks run.
func recordTiming(t *testing.T, start time.Time) {
	t.Helper()
	passed := !t.Failed()
	timingMu.Lock()
	timingResults = append(timingResults, timingEntry{
		name:    t.Name(),
		elapsed: time.Since(start),
		passed:  passed,
	})
	timingMu.Unlock()
}

// projectRoot walks up from the test binary's location to find go.mod.
//
// Postcondition: returns absolute path to the repository root.
func projectRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found")
		}
		dir = parent
	}
}

// freePort binds to :0, records the OS-assigned port, and releases the listener.
//
// Postcondition: returns an available port number.
func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port, nil
}

// buildBinary runs `go build -o outPath ./cmdPkg` in the project root.
//
// Postcondition: binary exists at outPath; returns error on build failure.
func buildBinary(root, cmdPkg, outPath string) error {
	cmd := exec.Command("go", "build", "-o", outPath, cmdPkg)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build %s: %w\n%s", cmdPkg, err, out)
	}
	return nil
}

// stopSubprocess sends SIGTERM to proc, waits up to 5 seconds for it to exit,
// then sends SIGKILL if it has not exited within the grace period.
//
// Precondition: proc must be a running subprocess started via startSubprocess.
func stopSubprocess(proc *exec.Cmd) {
	if proc == nil || proc.Process == nil {
		return
	}
	_ = proc.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_ = proc.Wait()
		close(done)
	}()
	select {
	case <-done:
		// process exited cleanly within grace period
	case <-time.After(5 * time.Second):
		_ = proc.Process.Kill()
	}
}

// stderrRingBuffer is a goroutine-safe fixed-capacity ring of the most
// recent stderr lines produced by a subprocess. It is used to surface a
// subprocess's actual error output when the harness gives up waiting for
// it (GH #234).
type stderrRingBuffer struct {
	mu       sync.Mutex
	capacity int
	lines    []string
}

func newStderrRingBuffer(capacity int) *stderrRingBuffer {
	return &stderrRingBuffer{capacity: capacity}
}

func (r *stderrRingBuffer) append(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.lines) >= r.capacity {
		r.lines = r.lines[1:]
	}
	r.lines = append(r.lines, line)
}

// Snapshot returns a joined copy of the lines currently in the buffer.
func (r *stderrRingBuffer) Snapshot() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.lines) == 0 {
		return ""
	}
	buf := make([]byte, 0, len(r.lines)*80)
	for i, ln := range r.lines {
		if i > 0 {
			buf = append(buf, '\n')
		}
		buf = append(buf, ln...)
	}
	return string(buf)
}

// startSubprocess starts a subprocess and captures its stderr into a ring
// buffer so the last N lines are available to diagnose failures (GH #234).
//
// Postcondition: process is running; stderrBuf.Snapshot() returns the most
// recent stderr lines; caller must call stopSubprocess on cleanup.
func startSubprocess(name, bin string, args []string, env []string) (*exec.Cmd, *stderrRingBuffer, error) {
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("%s stderr pipe: %w", name, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("starting %s: %w", name, err)
	}
	buf := newStderrRingBuffer(100)
	go func() {
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			buf.append(s.Text())
		}
	}()
	return cmd, buf, nil
}

// splitLines returns s split on newlines without trailing empties. Used to
// count captured stderr lines for diagnostic messages (GH #234).
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// pollPort dials addr every 200ms until it accepts or deadline elapses (REQ-ITS-2).
//
// Postcondition: returns nil when the port accepts; error on timeout.
func pollPort(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("port %s not ready after %s", addr, timeout)
}

// configData holds values for the e2e config template.
type configData struct {
	DBHost         string
	DBPort         int
	DBUser         string
	DBPassword     string
	DBName         string
	GameserverPort int
	FrontendPort   int
	HeadlessPort   int
}

// renderConfig renders testdata/e2e/config.yaml.tmpl into a temp file (REQ-ITS-11).
//
// Postcondition: temp file exists with rendered config; caller must os.Remove it.
func renderConfig(root string, data configData) (string, error) {
	tmplPath := filepath.Join(root, "testdata", "e2e", "config.yaml.tmpl")
	tmplBytes, err := os.ReadFile(tmplPath)
	if err != nil {
		return "", fmt.Errorf("reading config template: %w", err)
	}
	t, err := template.New("e2e-config").Parse(string(tmplBytes))
	if err != nil {
		return "", fmt.Errorf("parsing config template: %w", err)
	}
	f, err := os.CreateTemp("", "e2e-config-*.yaml")
	if err != nil {
		return "", fmt.Errorf("creating temp config: %w", err)
	}
	if err := t.Execute(f, data); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("rendering config template: %w", err)
	}
	_ = f.Close()
	return f.Name(), nil
}

// TestMain is the full e2e test lifecycle (REQ-ITS-1 through REQ-ITS-3).
func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
	overallStart := time.Now()
	ctx := context.Background()

	root, err := projectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: finding project root: %v\n", err)
		return 1
	}

	// Step 1: Start PostgreSQL container.
	fmt.Fprintf(os.Stderr, "e2e: starting postgres container...\n")
	pgReq := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "e2etest",
			"POSTGRES_PASSWORD": "e2etest",
			"POSTGRES_DB":       "e2etest",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: pgReq,
		Started:          true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: starting postgres: %v\n", err)
		return 1
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = container.Terminate(stopCtx)
	}()

	dbHost, err := container.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: container host: %v\n", err)
		return 1
	}
	dbMappedPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: container port: %v\n", err)
		return 1
	}
	dbPort := dbMappedPort.Int()

	// Step 2: Assign free ports.
	grpcPort, err := freePort()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: allocating grpc port: %v\n", err)
		return 1
	}
	frontendPort, err := freePort()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: allocating frontend port: %v\n", err)
		return 1
	}
	headlessPort, err := freePort()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: allocating headless port: %v\n", err)
		return 1
	}
	e2eState.HeadlessAddr = fmt.Sprintf("127.0.0.1:%d", headlessPort)

	// Step 3: Render config template.
	cfgFile, err := renderConfig(root, configData{
		DBHost:         dbHost,
		DBPort:         dbPort,
		DBUser:         "e2etest",
		DBPassword:     "e2etest",
		DBName:         "e2etest",
		GameserverPort: grpcPort,
		FrontendPort:   frontendPort,
		HeadlessPort:   headlessPort,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: rendering config: %v\n", err)
		return 1
	}
	defer os.Remove(cfgFile)

	// Step 4: Build binaries.
	binDir, err := os.MkdirTemp("", "e2e-bins-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: creating bin dir: %v\n", err)
		return 1
	}
	defer os.RemoveAll(binDir)

	fmt.Fprintf(os.Stderr, "e2e: building binaries...\n")
	gameserverBin := filepath.Join(binDir, "gameserver")
	frontendBin := filepath.Join(binDir, "frontend")
	migrateBin := filepath.Join(binDir, "migrate")
	seedBin := filepath.Join(binDir, "seed-claude-accounts")

	for _, b := range []struct{ pkg, out string }{
		{"./cmd/gameserver", gameserverBin},
		{"./cmd/frontend", frontendBin},
		{"./cmd/migrate", migrateBin},
		{"./cmd/seed-claude-accounts", seedBin},
	} {
		if err := buildBinary(root, b.pkg, b.out); err != nil {
			fmt.Fprintf(os.Stderr, "e2e: build %s: %v\n", b.pkg, err)
			return 1
		}
	}

	// Step 5: Apply migrations.
	fmt.Fprintf(os.Stderr, "e2e: applying migrations...\n")
	migrateCmd := exec.Command(migrateBin, "-config", cfgFile, "-migrations", filepath.Join(root, "migrations"))
	migrateCmd.Dir = root
	if out, err := migrateCmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: migration failed: %v\n%s\n", err, out)
		return 1
	}

	// Step 6: Start gameserver.
	// GH #234: the readiness timeout is configurable via MUD_E2E_READY_TIMEOUT
	// so slower machines (or cold content loads) do not spuriously fail.
	// Defaults to 60s, up from the previous hardcoded 30s that was too tight
	// for the ~40 content directories loaded at boot.
	readyTimeout := 60 * time.Second
	if raw := os.Getenv("MUD_E2E_READY_TIMEOUT"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			readyTimeout = parsed
		}
	}
	fmt.Fprintf(os.Stderr, "e2e: starting gameserver on :%d (ready timeout: %s)...\n", grpcPort, readyTimeout)
	gsProc, gsStderr, err := startSubprocess("gameserver", gameserverBin, []string{
		"-config", cfgFile,
		"-zones", filepath.Join(root, "content/zones"),
		"-npcs-dir", filepath.Join(root, "content/npcs"),
		"-conditions-dir", filepath.Join(root, "content/conditions"),
		"-script-root", filepath.Join(root, "content/scripts"),
		"-condition-scripts", filepath.Join(root, "content/scripts/conditions"),
		"-weapons-dir", filepath.Join(root, "content/weapons"),
		"-items-dir", filepath.Join(root, "content/items"),
		"-explosives-dir", filepath.Join(root, "content/explosives"),
		"-ai-dir", filepath.Join(root, "content/ai"),
		"-ai-scripts", filepath.Join(root, "content/scripts/ai"),
		"-armors-dir", filepath.Join(root, "content/armor"),
		"-precious-materials-dir", filepath.Join(root, "content/items/precious_materials"),
		"-jobs-dir", filepath.Join(root, "content/jobs"),
		"-loadouts-dir", filepath.Join(root, "content/loadouts"),
		"-skills", filepath.Join(root, "content/skills.yaml"),
		"-feats", filepath.Join(root, "content/feats.yaml"),
		"-class-features", filepath.Join(root, "content/class_features.yaml"),
		"-archetypes-dir", filepath.Join(root, "content/archetypes"),
		"-regions-dir", filepath.Join(root, "content/regions"),
		"-xp-config", filepath.Join(root, "content/xp_config.yaml"),
		"-tech-content-dir", filepath.Join(root, "content/technologies"),
		"-content-dir", filepath.Join(root, "content"),
		"-sets-dir", filepath.Join(root, "content/sets"),
		"-substances-dir", filepath.Join(root, "content/substances"),
		"-factions-dir", filepath.Join(root, "content/factions"),
		"-faction-config", filepath.Join(root, "content/faction_config.yaml"),
		"-materials-file", filepath.Join(root, "content/materials.yaml"),
		"-recipes-dir", filepath.Join(root, "content/recipes"),
		"-downtime-queue-limits", filepath.Join(root, "content/downtime_queue_limits.yaml"),
		"-quests-dir", filepath.Join(root, "content/quests"),
	}, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: starting gameserver: %v\n", err)
		return 1
	}
	defer stopSubprocess(gsProc)

	gsAddr := fmt.Sprintf("127.0.0.1:%d", grpcPort)
	if err := pollPort(gsAddr, readyTimeout); err != nil {
		if tail := gsStderr.Snapshot(); tail != "" {
			fmt.Fprintf(os.Stderr, "e2e: gameserver not ready: %v\ngameserver stderr (last %d lines):\n%s\n",
				err, len(splitLines(tail)), tail)
		} else {
			fmt.Fprintf(os.Stderr, "e2e: gameserver not ready: %v (no stderr output captured)\n", err)
		}
		return 1
	}

	// Step 7: Start frontend.
	fmt.Fprintf(os.Stderr, "e2e: starting frontend on :%d (headless :%d)...\n", frontendPort, headlessPort)
	feProc, feStderr, err := startSubprocess("frontend", frontendBin, []string{
		"-config", cfgFile,
		"-regions", filepath.Join(root, "content/regions"),
		"-teams", filepath.Join(root, "content/teams"),
		"-jobs", filepath.Join(root, "content/jobs"),
		"-archetypes", filepath.Join(root, "content/archetypes"),
		"-skills", filepath.Join(root, "content/skills.yaml"),
		"-feats", filepath.Join(root, "content/feats.yaml"),
		"-class-features", filepath.Join(root, "content/class_features.yaml"),
	}, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: starting frontend: %v\n", err)
		return 1
	}
	defer stopSubprocess(feProc)

	if err := pollPort(e2eState.HeadlessAddr, readyTimeout); err != nil {
		if tail := feStderr.Snapshot(); tail != "" {
			fmt.Fprintf(os.Stderr, "e2e: headless port not ready: %v\nfrontend stderr (last %d lines):\n%s\n",
				err, len(splitLines(tail)), tail)
		} else {
			fmt.Fprintf(os.Stderr, "e2e: headless port not ready: %v (no stderr output captured)\n", err)
		}
		return 1
	}

	// Step 8: Seed accounts (REQ-ITS-3).
	fmt.Fprintf(os.Stderr, "e2e: seeding claude accounts...\n")
	seedCmd := exec.Command(seedBin, "-config", cfgFile)
	seedCmd.Env = append(os.Environ(), "CLAUDE_ACCOUNT_PASSWORD=testpass123")
	seedCmd.Dir = root
	if out, err := seedCmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: seeding accounts: %v\n%s\n", err, out)
		return 1
	}

	fmt.Fprintf(os.Stderr, "e2e: stack ready [%s]\n", time.Since(overallStart))

	// Step 9: Run tests.
	result := m.Run()

	// Step 10: Print timing summary (REQ-ITS-10).
	timingMu.Lock()
	entries := timingResults
	timingMu.Unlock()
	sep := make([]byte, 75)
	for i := range sep {
		sep[i] = '-'
	}
	fmt.Fprintf(os.Stderr, "\n%-60s  %8s  %s\n", "Scenario", "ms", "Result")
	fmt.Fprintf(os.Stderr, "%s\n", sep)
	for _, e := range entries {
		status := "PASS"
		if !e.passed {
			status = "FAIL"
		}
		fmt.Fprintf(os.Stderr, "%-60s  %8d  %s\n", e.name, e.elapsed.Milliseconds(), status)
	}

	return result
}
