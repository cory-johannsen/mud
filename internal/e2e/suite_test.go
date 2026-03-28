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
	"testing"
	"text/template"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

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

// startSubprocess starts a subprocess and drains its stderr asynchronously.
//
// Postcondition: process is running; caller must call proc.Process.Kill() on cleanup.
func startSubprocess(name, bin string, args []string, env []string) (*exec.Cmd, error) {
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("%s stderr pipe: %w", name, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", name, err)
	}
	go func() {
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			_ = s.Text()
		}
	}()
	return cmd, nil
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
	fmt.Fprintf(os.Stderr, "e2e: starting gameserver on :%d...\n", grpcPort)
	gsProc, err := startSubprocess("gameserver", gameserverBin, []string{
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
	}, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: starting gameserver: %v\n", err)
		return 1
	}
	defer gsProc.Process.Kill()

	gsAddr := fmt.Sprintf("127.0.0.1:%d", grpcPort)
	if err := pollPort(gsAddr, 30*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: gameserver not ready: %v\n", err)
		return 1
	}

	// Step 7: Start frontend.
	fmt.Fprintf(os.Stderr, "e2e: starting frontend on :%d (headless :%d)...\n", frontendPort, headlessPort)
	feProc, err := startSubprocess("frontend", frontendBin, []string{
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
	defer feProc.Process.Kill()

	if err := pollPort(e2eState.HeadlessAddr, 30*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: headless port not ready: %v\n", err)
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
