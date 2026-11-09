//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	chromaui "github.com/githubixx/chroma-ui"
	"github.com/githubixx/la-memoria/adapters/configyaml"
	"github.com/githubixx/la-memoria/tests/testkit"
)

const composeTestVerifier = "$argon2id$v=19$m=65536,t=3,p=1$C9O9Mk4c2Zn/ZfyqxooQvA$HPd9+zneNgxejjew94cvrd8E8+MwuZcPhibe4x1i0vM"

func TestComposeDefaultInvocationPreservesRawCredentials(t *testing.T) {
	testkit.DockerAvailable(t)
	stack := testkit.CopyComposeBundle(t)
	stack.WriteEnvironment(t, composeTestVerifier)
	stack.Cleanup(t)
	dir := filepath.Join(stack.Root, "deploy", "compose")
	const password = "test$database#value"
	credentials := "BOOKMARKER_ADMIN_USERNAME=admin\nBOOKMARKER_ADMIN_PASSWORD_HASH=" + composeTestVerifier + "\nBOOKMARKER_DB_PASSWORD=" + password + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".env.credentials"), []byte(credentials), 0o600); err != nil {
		t.Fatal(err)
	}
	overrides, err := os.ReadFile(filepath.Join(dir, ".compose.env"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), overrides, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, cwd := range []string{stack.Root, dir} {
		t.Run(filepath.Base(cwd), func(t *testing.T) {
			compose := filepath.Join(dir, "docker-compose.yml")
			command := exec.Command("docker", "compose", "-p", stack.Project, "-f", compose, "run", "--rm", "--no-deps", "--entrypoint", "sh", "postgres", "-c", `printf '%s\n%s\n' "$BOOKMARKER_ADMIN_PASSWORD_HASH" "$BOOKMARKER_DB_PASSWORD"`)
			command.Dir = cwd
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "BOOKMARKER_") && !strings.HasPrefix(entry, "COMPOSE_") {
					command.Env = append(command.Env, entry)
				}
			}
			var stderr strings.Builder
			command.Stderr = &stderr
			output, err := command.Output()
			if err != nil {
				t.Fatalf("Compose container failed: %v", err)
			}
			if strings.Contains(stderr.String(), "variable is not set") {
				t.Fatal("Compose interpolated a credential as an environment variable")
			}
			if string(output) != composeTestVerifier+"\n"+password+"\n" {
				t.Fatal("container did not receive intact raw credentials")
			}
		})
	}
}

func TestComposeInitializerImportFailuresRetryAndRotation(t *testing.T) {
	catalog, err := chromaui.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	theme := catalog.Themes()[0].ID
	dir := t.TempDir()
	target := filepath.Join(dir, "volume", "config.yaml")
	if err := os.Mkdir(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	template := filepath.Join(dir, "template.yaml")
	if err := os.WriteFile(template, []byte(concurrentConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "export.yaml")
	env := configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "test"}
	init := func(importPath, username string) error {
		return configyaml.InitializeCompose(context.Background(), target, template, importPath, username, composeTestVerifier, env)
	}
	for _, mode := range []string{"missing", "invalid", "unreadable"} {
		t.Run(mode, func(t *testing.T) {
			_ = os.Remove(source)
			if mode == "invalid" {
				if err := os.WriteFile(source, []byte("invalid: [yaml"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "unreadable" {
				if err := os.WriteFile(source, []byte(concurrentConfig), 0o000); err != nil {
					t.Fatal(err)
				}
			}
			if err := init(source, "rotated"); err == nil {
				t.Fatal("bad explicit import succeeded")
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatalf("failed import initialized target: %v", err)
			}
		})
	}
	_ = os.Remove(source)
	if err := os.WriteFile(source, []byte(strings.Replace(concurrentConfig, "page_title: Bookmarker", "page_title: Imported title", 1)+"\ntheme: "+theme+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := init(source, "rotated"); err != nil {
		t.Fatalf("retry import: %v", err)
	}
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := configyaml.Load(target, env)
	if err != nil || loaded.Branding.PageTitle != "Imported title" || loaded.Theme != theme || loaded.Administrator.Username != "rotated" {
		t.Fatalf("import did not preserve settings or refresh credentials: %v", err)
	}
	if err := init(source, "other"); err == nil {
		t.Fatal("import overwrote initialized volume")
	}
	after, err := os.ReadFile(target)
	if err != nil || string(after) != string(before) {
		t.Fatal("refused import changed bytes")
	}
	if err := init("", "rotated-again"); err != nil {
		t.Fatalf("credential rotation: %v", err)
	}
	loaded, err = configyaml.Load(target, env)
	if err != nil || loaded.Branding.PageTitle != "Imported title" || loaded.Theme != theme || loaded.Administrator.Username != "rotated-again" {
		t.Fatalf("rotation did not preserve settings: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions: %v %v", info, err)
	}
	fresh := filepath.Join(dir, "fresh.yaml")
	if err := configyaml.InitializeCompose(context.Background(), fresh, template, "", "fresh", composeTestVerifier, env); err != nil {
		t.Fatalf("fresh initialization: %v", err)
	}
}

func TestComposeExplicitImportFailuresDoNotInitializeVolume(t *testing.T) {
	testkit.DockerAvailable(t)
	stack := testkit.CopyComposeBundle(t)
	stack.WriteEnvironment(t, composeTestVerifier)
	stack.Cleanup(t)
	importDir := filepath.Join(stack.Root, "import-test")
	if err := os.Mkdir(importDir, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(importDir, "config.yaml")
	override := filepath.Join(stack.Root, "import-test.yaml")
	content := fmt.Sprintf("services:\n  bookmarker:\n    environment:\n      BOOKMARKER_CONFIG_IMPORT_PATH: /import/config.yaml\n    volumes:\n      - type: bind\n        source: %q\n        target: /import\n        read_only: true\n", importDir)
	if err := os.WriteFile(override, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	stack.Run(t, "build", "bookmarker")
	for _, mode := range []string{"missing", "invalid", "unreadable"} {
		t.Run(mode, func(t *testing.T) {
			_ = os.Remove(source)
			switch mode {
			case "invalid":
				if err := os.WriteFile(source, []byte("invalid: [yaml"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				if err := os.WriteFile(source, []byte(concurrentConfig), 0o000); err != nil {
					t.Fatal(err)
				}
			}
			output, err := stack.Output("-f", override, "run", "--rm", "--no-deps", "bookmarker", "true")
			if err == nil || strings.Contains(output, composeTestVerifier) || strings.Contains(output, "bookmarker-test") {
				t.Fatal("invalid import succeeded or leaked credentials")
			}
			if output, err := stack.Output("run", "--rm", "--no-deps", "--entrypoint", "test", "bookmarker", "!", "-e", "/run/bookmarker/config.yaml"); err != nil {
				t.Fatalf("failed import left initialized file: %v %s", err, output)
			}
		})
	}
	_ = os.Remove(source)
	if err := os.WriteFile(source, []byte(concurrentConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	stack.Run(t, "-f", override, "run", "--rm", "--no-deps", "bookmarker", "true")
	if output, err := stack.Output("-f", override, "run", "--rm", "--no-deps", "bookmarker", "true"); err == nil || strings.Contains(output, composeTestVerifier) {
		t.Fatal("initialized volume accepted a second import or leaked credentials")
	}
}
func TestComposeConfigExportImportRestartAndRecreation(t *testing.T) {
	catalog, err := chromaui.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	theme := catalog.Themes()[0].ID
	testkit.DockerAvailable(t)
	stack := testkit.CopyComposeBundle(t)
	stack.WriteEnvironment(t, composeTestVerifier)
	stack.Cleanup(t)
	stack.Run(t, "up", "--build", "--detach")
	testkit.WaitForHTTP(t, fmt.Sprintf("http://127.0.0.1:%s/bookmarks", stack.Port))
	export := filepath.Join(stack.Root, "old-config-export.yaml")
	stack.Run(t, "cp", "bookmarker:/run/bookmarker/config.yaml", export)
	data, err := os.ReadFile(export)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "page_title: Bookmarker", "page_title: Exported title", 1) + "\ntheme: " + theme + "\n")
	data = []byte(strings.Replace(string(data), "username: admin", "username: old-admin", 1))
	if err := os.WriteFile(export, data, 0o600); err != nil {
		t.Fatal(err)
	}
	stack.Run(t, "cp", export, "bookmarker:/run/bookmarker/config.yaml")
	stack.Run(t, "cp", "bookmarker:/run/bookmarker/config.yaml", export)
	// Export exists outside the isolated volume before its removal.
	stack.Run(t, "down", "--volumes")
	override := filepath.Join(stack.Root, "import-override.yaml")
	overrideYAML := fmt.Sprintf("services:\n  bookmarker:\n    environment:\n      BOOKMARKER_CONFIG_IMPORT_PATH: /import/config.yaml\n    volumes:\n      - type: bind\n        source: %q\n        target: /import/config.yaml\n        read_only: true\n", export)
	if err := os.WriteFile(override, []byte(overrideYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	stack.Run(t, "-f", override, "up", "--detach")
	testkit.WaitForHTTP(t, fmt.Sprintf("http://127.0.0.1:%s/bookmarks", stack.Port))
	check := func(expectedUsername string) {
		t.Helper()
		file := filepath.Join(stack.Root, "observed.yaml")
		stack.Run(t, "cp", "bookmarker:/run/bookmarker/config.yaml", file)
		contents, err := os.ReadFile(file)
		if err != nil || !strings.Contains(string(contents), "Exported title") || !strings.Contains(string(contents), theme) || !strings.Contains(string(contents), "username: "+expectedUsername) {
			t.Fatal("imported configuration did not persist")
		}
		mode := strings.TrimSpace(stack.Run(t, "exec", "-T", "bookmarker", "stat", "-c", "%a", "/run/bookmarker/config.yaml"))
		if mode != "600" {
			t.Fatalf("config mode %q, want 600", mode)
		}
	}
	check("admin")
	if output, err := stack.Output("-f", override, "run", "--rm", "--no-deps", "bookmarker"); err == nil || strings.Contains(output, composeTestVerifier) || strings.Contains(output, "bookmarker-test") {
		t.Fatal("reimport into initialized volume did not fail safely")
	}
	stack.Run(t, "up", "--detach", "--force-recreate", "bookmarker")
	testkit.WaitForHTTP(t, fmt.Sprintf("http://127.0.0.1:%s/bookmarks", stack.Port))
	check("admin")
	envPath := filepath.Join(stack.Root, "deploy", "compose", ".env.credentials")
	env, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	env = []byte(strings.Replace(string(env), "BOOKMARKER_ADMIN_USERNAME=admin", "BOOKMARKER_ADMIN_USERNAME=rotated-admin", 1))
	if err := os.WriteFile(envPath, env, 0o600); err != nil {
		t.Fatal(err)
	}
	// Recreate applies changed deployment environment and rotates credentials.
	stack.Run(t, "up", "--detach", "--force-recreate", "bookmarker")
	testkit.WaitForHTTP(t, fmt.Sprintf("http://127.0.0.1:%s/bookmarks", stack.Port))
	check("rotated-admin")
	stack.Run(t, "restart", "bookmarker")
	testkit.WaitForHTTP(t, fmt.Sprintf("http://127.0.0.1:%s/bookmarks", stack.Port))
	check("rotated-admin")
}

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

	bookmarkID := fmt.Sprintf("compose-bookmark-%d", time.Now().UnixNano())
	screenshotFile := fmt.Sprintf("compose-%d.png", time.Now().UnixNano())

	stack.Run(t, "up", "--build", "--detach")
	testkit.WaitForHTTP(t, fmt.Sprintf("http://127.0.0.1:%s/bookmarks", stack.Port))
	stack.Run(t, "exec", "-T", "postgres", "psql", "-U", "bookmarker", "-d", "bookmarker", "-c", fmt.Sprintf("INSERT INTO bookmarks (id, url, description, created_at, updated_at) VALUES ('%s', 'https://example.test', 'Compose persistence check', now(), now())", bookmarkID))
	stack.Run(t, "exec", "-T", "postgres", "psql", "-U", "bookmarker", "-d", "bookmarker", "-c", fmt.Sprintf("INSERT INTO screenshots (id, bookmark_id, storage_key, captured_url, captured_at, byte_size) VALUES ('%s', '%s', '%s', 'https://example.test', now(), 10)", bookmarkID+"-screenshot", bookmarkID, screenshotFile))
	stack.Run(t, "exec", "-T", "bookmarker", "sh", "-c", fmt.Sprintf("printf screenshot > /var/lib/bookmarker/screenshots/%s", screenshotFile))
	stack.Run(t, "down")
	stack.Run(t, "up", "--detach")
	testkit.WaitForHTTP(t, fmt.Sprintf("http://127.0.0.1:%s/bookmarks", stack.Port))
	if output := stack.Run(t, "exec", "-T", "postgres", "psql", "-U", "bookmarker", "-d", "bookmarker", "-tAc", fmt.Sprintf("SELECT description FROM bookmarks WHERE id = '%s'", bookmarkID)); !strings.Contains(output, "Compose persistence check") {
		t.Fatalf("persisted bookmark query = %q", output)
	}
	if output, err := stack.Output("exec", "-T", "bookmarker", "test", "-s", fmt.Sprintf("/var/lib/bookmarker/screenshots/%s", screenshotFile)); err != nil {
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
