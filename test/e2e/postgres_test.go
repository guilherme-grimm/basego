package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestCreate_PostgresRepoAgainstRealDB scaffolds a postgres project, starts
// postgres with the generated schema mounted the same way docker-compose
// mounts it, and runs the generated repo test against it. Skipped under
// -short or without a reachable docker daemon.
func TestCreate_PostgresRepoAgainstRealDB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping postgres test in -short mode")
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
	create := exec.Command(basego, "create", "--module=example.com/demo", "--db=postgres", "demo")
	create.Dir = root
	out, err := create.CombinedOutput()
	require.NoError(t, err, "basego create:\n%s", out)
	target := filepath.Join(root, "demo")

	schema := filepath.Join(target, "internal/resource/database/postgres/schema")
	out, err = exec.Command(docker, "run", "-d", "-p", "127.0.0.1::5432",
		"-e", "POSTGRES_USER=user", "-e", "POSTGRES_PASSWORD=pass", "-e", "POSTGRES_DB=demo",
		"-v", schema+":/docker-entrypoint-initdb.d:ro", "postgres:16").Output()
	require.NoError(t, err, "docker run postgres")
	id := strings.TrimSpace(string(out))
	t.Cleanup(func() { _ = exec.Command(docker, "rm", "-f", id).Run() })

	out, err = exec.Command(docker, "port", id, "5432/tcp").CombinedOutput()
	require.NoError(t, err, "docker port:\n%s", out)
	addr := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	waitPostgresReady(t, docker, id)

	test := exec.Command(goBin, "test", "-count=1", "-run", "TestExampleRepo", "-v", "./internal/resource/database/postgres/")
	test.Dir = target
	test.Env = append(os.Environ(), fmt.Sprintf("TEST_POSTGRES_DSN=postgres://user:pass@%s/demo?sslmode=disable", addr))
	out, err = test.CombinedOutput()
	require.NoError(t, err, "generated repo test:\n%s", out)
	require.NotContains(t, string(out), "SKIP", "repo test skipped:\n%s", out)
}

// waitPostgresReady waits until the init scripts have run and the final
// server accepts connections. The image runs init scripts on a temporary
// server first, so pg_isready alone can pass before the schema exists.
func waitPostgresReady(t *testing.T, docker, id string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		logs, _ := exec.Command(docker, "logs", id).CombinedOutput()
		if strings.Contains(string(logs), "PostgreSQL init process complete") &&
			exec.Command(docker, "exec", id, "pg_isready", "-U", "user", "-d", "demo").Run() == nil {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.FailNow(t, "postgres never became ready")
}
