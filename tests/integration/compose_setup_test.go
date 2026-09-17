//go:build integration

package integration_test

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/githubixx/la-memoria/tests/testkit"
)

const composeTestVerifier = "$argon2id$v=19$m=65536,t=3,p=1$C9O9Mk4c2Zn/ZfyqxooQvA$HPd9+zneNgxejjew94cvrd8E8+MwuZcPhibe4x1i0vM"

func TestComposeStartsAfterPostgreSQLHealthAndServesBookmarker(t *testing.T) {
	testkit.DockerAvailable(t)
	stack := testkit.CopyComposeBundle(t)
	stack.Cleanup(t)
	stack.WriteEnvironment(t, composeTestVerifier)

	stack.Run(t, "up", "--build", "--detach")
	testkit.WaitForHTTP(t, fmt.Sprintf("http://127.0.0.1:%s/bookmarks", stack.Port))

	status := stack.Run(t, "ps")
	if !strings.Contains(status, "healthy") {
		t.Fatalf("Compose service status = %q, want healthy services", status)
	}
}

func TestComposeRetainsPostgreSQLDataAcrossBookmarkerRestart(t *testing.T) {
	testkit.DockerAvailable(t)
	stack := testkit.CopyComposeBundle(t)
	stack.Cleanup(t)
	stack.WriteEnvironment(t, composeTestVerifier)

	stack.Run(t, "up", "--build", "--detach")
	testkit.WaitForHTTP(t, fmt.Sprintf("http://127.0.0.1:%s/bookmarks", stack.Port))
	stack.Run(t, "exec", "-T", "postgres", "psql", "-U", "bookmarker", "-d", "bookmarker", "-c", "INSERT INTO bookmarks (id, url, description, created_at, updated_at) VALUES ('compose-bookmark', 'https://example.test', 'Compose persistence check', now(), now())")
	stack.Run(t, "exec", "-T", "postgres", "psql", "-U", "bookmarker", "-d", "bookmarker", "-c", "INSERT INTO screenshots (id, bookmark_id, storage_key, captured_url, captured_at, byte_size) VALUES ('compose-screenshot', 'compose-bookmark', 'compose.png', 'https://example.test', now(), 10)")
	stack.Run(t, "exec", "-T", "bookmarker", "sh", "-c", "printf screenshot > /var/lib/bookmarker/screenshots/compose.png")
	stack.Run(t, "down")
	stack.Run(t, "up", "--detach")
	testkit.WaitForHTTP(t, fmt.Sprintf("http://127.0.0.1:%s/bookmarks", stack.Port))
	if output := stack.Run(t, "exec", "-T", "postgres", "psql", "-U", "bookmarker", "-d", "bookmarker", "-tAc", "SELECT description FROM bookmarks WHERE id = 'compose-bookmark'"); !strings.Contains(output, "Compose persistence check") {
		t.Fatalf("persisted bookmark query = %q", output)
	}
	if output, err := stack.Output("exec", "-T", "bookmarker", "test", "-s", "/var/lib/bookmarker/screenshots/compose.png"); err != nil {
		t.Fatalf("persisted screenshot check: %v\n%s", err, output)
	}
}

func TestComposeCopiedBundleHonorsHostPortAndNamedVolumeOverrides(t *testing.T) {
	testkit.DockerAvailable(t)
	stack := testkit.CopyComposeBundle(t)
	stack.Cleanup(t)
	stack.WriteEnvironment(t, composeTestVerifier)

	stack.Run(t, "up", "--build", "--detach")
	testkit.WaitForHTTP(t, fmt.Sprintf("http://127.0.0.1:%s/bookmarks", stack.Port))
	status := stack.Run(t, "ps")
	if !strings.Contains(status, "healthy") {
		t.Fatalf("Compose service status = %q, want healthy services", status)
	}
}

func TestComposeRejectsMissingRequiredEnvironmentValue(t *testing.T) {
	testkit.DockerAvailable(t)
	stack := testkit.CopyComposeBundle(t)
	stack.Cleanup(t)

	output, err := stack.Output("run", "--rm", "--no-deps", "bookmarker")
	if err == nil {
		t.Fatal("Bookmarker started without required configuration")
	}
	if !strings.Contains(output, "missing required environment setting") || strings.Contains(output, "BOOKMARKER_DB_PASSWORD=") {
		t.Fatalf("missing environment diagnostic = %q", output)
	}
}

func TestComposeReportsUnavailablePostgreSQLWithoutExposingSecrets(t *testing.T) {
	testkit.DockerAvailable(t)
	stack := testkit.CopyComposeBundle(t)
	stack.Cleanup(t)
	stack.WriteEnvironment(t, composeTestVerifier)

	output, err := stack.Output("run", "--rm", "--no-deps", "bookmarker")
	if err == nil {
		t.Fatal("Bookmarker started without PostgreSQL")
	}
	if !strings.Contains(output, "connect PostgreSQL") || strings.Contains(output, "bookmarker-test") {
		t.Fatalf("unavailable dependency diagnostic = %q", output)
	}
}

func TestComposeReportsHostPortConflict(t *testing.T) {
	testkit.DockerAvailable(t)
	stack := testkit.CopyComposeBundle(t)
	stack.Cleanup(t)
	stack.WriteEnvironment(t, composeTestVerifier)
	listener, err := net.Listen("tcp", "127.0.0.1:"+stack.Port)
	if err != nil {
		t.Fatalf("occupy requested host port: %v", err)
	}
	defer listener.Close()

	output, err := stack.Output("up", "--build", "--detach", "--no-deps", "bookmarker")
	if err == nil {
		t.Fatal("Compose started Bookmarker despite occupied host port")
	}
	if !strings.Contains(strings.ToLower(output), "port") || !strings.Contains(strings.ToLower(output), "address") {
		t.Fatalf("host-port conflict diagnostic = %q", output)
	}
}

func TestComposeHashServiceWorksWithoutEnvironmentFile(t *testing.T) {
	testkit.DockerAvailable(t)
	stack := testkit.CopyComposeBundle(t)
	stack.Cleanup(t)

	output, err := stack.OutputWithInput("correct horse battery staple\n", "run", "--rm", "-T", "bookmarker-hash")
	if err != nil {
		t.Fatalf("run bootstrap hash service: %v\n%s", err, output)
	}
	if !strings.Contains(output, "$argon2id$v=19$m=65536,t=3,p=1$") || strings.Contains(output, "correct horse battery staple") {
		t.Fatalf("bootstrap hash output = %q, want verifier without plaintext password", output)
	}
}
