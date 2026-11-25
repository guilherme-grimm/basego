package e2e_test

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreate_ImageServesMetrics builds the generated project's Docker image,
// runs it, and scrapes /metrics. Unit tests and `go build` on the host can't
// catch Dockerfile drift (e.g. a base image older than go.mod requires); this
// does. Skipped under -short or without a reachable docker daemon.
func TestCreate_ImageServesMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docker image test in -short mode")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go binary not in PATH")
	}
	docker, err := exec.LookPath("docker")
	if err != nil || exec.Command(docker, "info").Run() != nil {
		t.Skip("docker daemon not available")
	}

	basego := buildBasego(t, goBin)
	root := t.TempDir()
	create := exec.Command(basego, "create", "--module=example.com/demo", "demo")
	create.Dir = root
	out, err := create.CombinedOutput()
	require.NoError(t, err, "basego create:\n%s", out)

	tag := fmt.Sprintf("basego-e2e-%d", time.Now().UnixNano())
	build := exec.Command(docker, "build", "-q", "-t", tag, ".")
	build.Dir = filepath.Join(root, "demo")
	out, err = build.CombinedOutput()
	require.NoError(t, err, "docker build:\n%s", out)
	t.Cleanup(func() { _ = exec.Command(docker, "rmi", "-f", tag).Run() })

	out, err = exec.Command(docker, "run", "-d", "-p", "127.0.0.1::8080", tag).CombinedOutput()
	require.NoError(t, err, "docker run:\n%s", out)
	id := strings.TrimSpace(string(out))
	t.Cleanup(func() { _ = exec.Command(docker, "rm", "-f", id).Run() })

	out, err = exec.Command(docker, "port", id, "8080/tcp").CombinedOutput()
	require.NoError(t, err, "docker port:\n%s", out)
	addr := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])

	body := scrape(t, "http://"+addr+"/metrics")
	assert.Contains(t, body, "go_goroutines")
	assert.Contains(t, body, "process_start_time_seconds")
}

// scrape polls url until it returns 200 or 30s pass, returning the body.
func scrape(t *testing.T, url string) string {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := http.Get(url)
		if err == nil {
			b, rerr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if rerr == nil && resp.StatusCode == http.StatusOK {
				return string(b)
			}
		}
		if time.Now().After(deadline) {
			require.FailNow(t, "metrics endpoint never became ready", "url %s, last err: %v", url, err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
