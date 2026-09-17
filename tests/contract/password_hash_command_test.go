//go:build contract

package contract_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestPasswordHashCommandWritesOnlyVerifierToStandardOutput(t *testing.T) {
	command := exec.Command("go", "run", "../../cmd/bookmarker-hash")
	command.Stdin = strings.NewReader("correct horse battery staple\n")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("run password hash command: %v", err)
	}
	verifier := string(output)
	if !strings.HasPrefix(verifier, "$argon2id$v=19$m=65536,t=3,p=1$") || !strings.HasSuffix(verifier, "\n") || strings.Count(verifier, "\n") != 1 {
		t.Fatalf("stdout = %q, want exactly one newline-terminated verifier", verifier)
	}
	if strings.Contains(verifier, "correct horse") {
		t.Fatalf("stdout disclosed password: %q", verifier)
	}
}

func TestPasswordHashCommandRejectsEmptyPassword(t *testing.T) {
	command := exec.Command("go", "run", "../../cmd/bookmarker-hash")
	command.Stdin = strings.NewReader("\n")
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("empty password command succeeded")
	}
	if strings.Contains(string(output), "\n$argon2id$") || strings.Contains(string(output), "\n\n") {
		t.Fatalf("empty password command emitted verifier: %q", output)
	}
}
