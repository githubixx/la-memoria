package testkit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type ComposeStack struct {
	Project string
	Root    string
	Port    string
}

func CopyComposeBundle(t *testing.T) *ComposeStack {
	t.Helper()
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	root := filepath.Join(t.TempDir(), "bookmarker")
	if output, err := exec.Command("cp", "-a", repositoryRoot, root).CombinedOutput(); err != nil {
		t.Fatalf("copy source checkout: %v: %s", err, output)
	}
	project := "bookmarker_" + strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_")
	return &ComposeStack{Project: project, Root: root, Port: freeTCPPort(t)}
}

func (stack *ComposeStack) WriteEnvironment(t *testing.T, verifier string) {
	t.Helper()
	contents := fmt.Sprintf("BOOKMARKER_ADMIN_USERNAME=admin\nBOOKMARKER_ADMIN_PASSWORD_HASH=%s\nBOOKMARKER_DB_PASSWORD=bookmarker-test\n", verifier)
	path := filepath.Join(stack.Root, "deploy", "compose", ".env")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write Compose environment: %v", err)
	}
	composeContents := fmt.Sprintf("BOOKMARKER_HOST_PORT=%s\nBOOKMARKER_POSTGRES_VOLUME=%s-postgres\nBOOKMARKER_SCREENSHOTS_VOLUME=%s-screenshots\n", stack.Port, stack.Project, stack.Project)
	composePath := filepath.Join(stack.Root, "deploy", "compose", ".compose.env")
	if err := os.WriteFile(composePath, []byte(composeContents), 0o600); err != nil {
		t.Fatalf("write Compose interpolation environment: %v", err)
	}
}

func (stack *ComposeStack) Run(t *testing.T, arguments ...string) string {
	t.Helper()
	command := exec.Command("docker", append([]string{"compose", "--env-file", "deploy/compose/.compose.env", "-p", stack.Project, "-f", "deploy/compose/docker-compose.yml"}, arguments...)...)
	command.Dir = stack.Root
	output, err := command.CombinedOutput()
	if err != nil {
		logs, _ := stack.Output("logs", "--no-color")
		t.Fatalf("run docker compose %s: %v\n%s\n%s", strings.Join(arguments, " "), err, output, logs)
	}
	return string(output)
}

func (stack *ComposeStack) Output(arguments ...string) (string, error) {
	return stack.OutputWithInput("", arguments...)
}

func (stack *ComposeStack) OutputWithInput(input string, arguments ...string) (string, error) {
	command := exec.Command("docker", append([]string{"compose", "--env-file", "deploy/compose/.compose.env", "-p", stack.Project, "-f", "deploy/compose/docker-compose.yml"}, arguments...)...)
	command.Dir = stack.Root
	command.Stdin = strings.NewReader(input)
	output, err := command.CombinedOutput()
	return string(output), err
}

func (stack *ComposeStack) Cleanup(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		output, err := stack.Output("down", "--volumes", "--remove-orphans")
		if err != nil {
			t.Errorf("stop Docker Compose stack: %v\n%s", err, output)
		}
	})
}

func WaitForHTTP(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		response, err := (&httpClient).Get(address)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode < 500 {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("Bookmarker did not become reachable at %s", address)
}

var httpClient = http.Client{Timeout: 2 * time.Second}

func freeTCPPort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve TCP port: %v", err)
	}
	defer listener.Close()
	return fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
}

func DockerAvailable(t *testing.T) {
	t.Helper()
	if err := exec.CommandContext(context.Background(), "docker", "info").Run(); err != nil {
		t.Skipf("Docker is unavailable: %v", err)
	}
}
