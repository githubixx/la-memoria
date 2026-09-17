//go:build contract

package contract_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const composeDirectory = "../../deploy/compose"

func TestComposeBundleContainsRequiredNonSecretArtifacts(t *testing.T) {
	for _, name := range []string{"Dockerfile", "docker-compose.yml", "config.yaml.tmpl", "entrypoint.sh", ".env.example"} {
		if _, err := os.Stat(filepath.Join(composeDirectory, name)); err != nil {
			t.Fatalf("deployment bundle is missing %s: %v", name, err)
		}
	}

	environment := readComposeArtifact(t, ".env.example")
	for _, name := range []string{"BOOKMARKER_ADMIN_USERNAME", "BOOKMARKER_ADMIN_PASSWORD_HASH", "BOOKMARKER_DB_PASSWORD", "BOOKMARKER_HOST_PORT", "BOOKMARKER_POSTGRES_VOLUME", "BOOKMARKER_SCREENSHOTS_VOLUME"} {
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
	for _, value := range []string{"BOOKMARKER_ADMIN_USERNAME", "BOOKMARKER_ADMIN_PASSWORD_HASH", "BOOKMARKER_DB_PASSWORD", "chmod 600", "exec"} {
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
