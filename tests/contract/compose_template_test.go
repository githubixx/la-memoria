//go:build contract

package contract_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const composeDirectory = "../../deploy/compose"

func TestComposeKeepsRawCredentialsOutOfInterpolation(t *testing.T) {
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		t.Skipf("Docker Compose is unavailable: %v", err)
	}
	root := t.TempDir()
	dir := filepath.Join(root, "deploy", "compose")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	compose, err := os.ReadFile(filepath.Join(composeDirectory, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), compose, 0o600); err != nil {
		t.Fatal(err)
	}
	const verifier = "$argon2id$v=19$m=65536,t=3,p=1$c2FsdA$aGFzaA"
	const dbPassword = "literal$database#value"
	files := map[string]string{
		".env":             "BOOKMARKER_HOST_PORT=18088\nBOOKMARKER_CONFIG_VOLUME=isolated-config\n",
		".env.credentials": "BOOKMARKER_ADMIN_USERNAME=admin\nBOOKMARKER_ADMIN_PASSWORD_HASH=" + verifier + "\nBOOKMARKER_DB_PASSWORD=" + dbPassword + "\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, cwd := range []string{root, dir} {
		t.Run(filepath.Base(cwd), func(t *testing.T) {
			command := exec.Command("docker", "compose", "-f", filepath.Join(dir, "docker-compose.yml"), "config", "--format", "json")
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
				t.Fatalf("compose config failed: %v", err)
			}
			if stderr.Len() != 0 {
				t.Fatalf("compose config emitted warnings: %s", stderr.String())
			}
			var config struct {
				Services map[string]struct {
					Environment map[string]string `json:"environment"`
					Ports       []struct {
						Published string `json:"published"`
					} `json:"ports"`
				} `json:"services"`
				Volumes map[string]struct {
					Name string `json:"name"`
				} `json:"volumes"`
			}
			if err := json.Unmarshal(output, &config); err != nil {
				t.Fatalf("decode compose configuration: %v", err)
			}
			for _, service := range []string{"bookmarker", "postgres"} {
				env := config.Services[service].Environment
				// Compose escapes dollar signs in rendered configuration.
				if strings.ReplaceAll(env["BOOKMARKER_ADMIN_PASSWORD_HASH"], "$$", "$") != verifier ||
					strings.ReplaceAll(env["BOOKMARKER_DB_PASSWORD"], "$$", "$") != dbPassword {
					t.Errorf("%s did not receive intact raw credentials", service)
				}
			}
			if ports := config.Services["bookmarker"].Ports; len(ports) != 1 || ports[0].Published != "18088" {
				t.Errorf("interpolation did not preserve the configured host port")
			}
			if config.Volumes["configuration-data"].Name != "isolated-config" {
				t.Error("interpolation did not preserve the configured volume name")
			}
		})
	}
}

func TestComposeBundleContainsRequiredNonSecretArtifacts(t *testing.T) {
	for _, name := range []string{"Dockerfile", "docker-compose.yml", "config.yaml.tmpl", "entrypoint.sh", ".env.example", ".env.credentials.example"} {
		if _, err := os.Stat(filepath.Join(composeDirectory, name)); err != nil {
			t.Fatalf("deployment bundle is missing %s: %v", name, err)
		}
	}

	environment := readComposeArtifact(t, ".env.example") + readComposeArtifact(t, ".env.credentials.example")
	for _, name := range []string{"BOOKMARKER_ADMIN_USERNAME", "BOOKMARKER_ADMIN_PASSWORD_HASH", "BOOKMARKER_DB_PASSWORD", "BOOKMARKER_HOST_PORT", "BOOKMARKER_POSTGRES_VOLUME", "BOOKMARKER_SCREENSHOTS_VOLUME", "BOOKMARKER_CONFIG_VOLUME"} {
		if !strings.Contains(environment, name+"=") {
			t.Errorf("environment example is missing %s", name)
		}
	}
	if strings.Contains(environment, "$argon2id$") || strings.Contains(environment, "password=") {
		t.Fatal("environment example must not contain runnable credentials")
	}
}

func TestComposeTemplateKeepsPostgreSQLPrivateAndUsesNamedVolumes(t *testing.T) {
	compose := readComposeArtifact(t, "docker-compose.yml")
	for _, value := range []string{
		"bookmarker-hash:",
		"postgres:",
		"${BOOKMARKER_HOST_PORT:-8080}:8080",
		"${BOOKMARKER_POSTGRES_VOLUME:-bookmarker-postgres-data}",
		"${BOOKMARKER_SCREENSHOTS_VOLUME:-bookmarker-screenshots}",
		"condition: service_healthy",
	} {
		if !strings.Contains(compose, value) {
			t.Errorf("Compose template is missing %q", value)
		}
	}
	postgresStart := strings.Index(compose, "  postgres:\n")
	if postgresStart < 0 {
		t.Fatal("Compose template is missing the postgres service")
	}
	postgresSection := compose[postgresStart:]
	if nextService := strings.Index(postgresSection[len("  postgres:\n"):], "\n  "); nextService >= 0 {
		postgresSection = postgresSection[:len("  postgres:\n")+nextService]
	}
	if strings.Contains(postgresSection, "ports:") {
		t.Fatal("PostgreSQL must not publish a host port")
	}
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	for _, name := range []string{"BOOKMARKER_ADMIN_USERNAME", "BOOKMARKER_ADMIN_PASSWORD_HASH", "BOOKMARKER_DB_PASSWORD", "BOOKMARKER_HOST_PORT", "BOOKMARKER_POSTGRES_VOLUME", "BOOKMARKER_SCREENSHOTS_VOLUME"} {
		if !strings.Contains(string(readme), name) {
			t.Errorf("README does not document %s", name)
		}
	}
}

func TestEntrypointRejectsUnsafeConfigurationValuesWithoutLoggingSecrets(t *testing.T) {
	entrypoint := readComposeArtifact(t, "entrypoint.sh")
	for _, value := range []string{"BOOKMARKER_ADMIN_USERNAME", "BOOKMARKER_ADMIN_PASSWORD_HASH", "BOOKMARKER_DB_PASSWORD", "bookmarker-config-init", "exec"} {
		if !strings.Contains(entrypoint, value) {
			t.Errorf("entrypoint is missing %q", value)
		}
	}
	if strings.Contains(entrypoint, "echo \"$BOOKMARKER_ADMIN_PASSWORD_HASH\"") || strings.Contains(entrypoint, "echo \"$BOOKMARKER_DB_PASSWORD\"") {
		t.Fatal("entrypoint must not log secret values")
	}
}

func readComposeArtifact(t *testing.T, name string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(composeDirectory, name))
	if err != nil {
		t.Fatalf("read deployment bundle artifact %s: %v", name, err)
	}
	return string(contents)
}
