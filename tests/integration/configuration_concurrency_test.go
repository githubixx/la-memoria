//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/configyaml"
)

const concurrentConfig = `administrator:
  username: admin
  password_hash: $argon2id$v=19$m=65536,t=3,p=1$c2FsdA$aGFzaA
branding:
  page_title: Bookmarker
database:
  host: 127.0.0.1
  port: 5432
  name: bookmarker
  user: bookmarker
  password_env: BOOKMARKER_DB_PASSWORD
  tls_mode: disable
screenshots:
  root: /tmp/bookmarker-screenshots
`

func TestConfigurationConcurrentProcessHelper(t *testing.T) {
	path, field := os.Getenv("TEST_CONFIGURATION_PATH"), os.Getenv("TEST_CONFIGURATION_FIELD")
	if path == "" {
		return
	}
	store := configyaml.NewStore(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "test"})
	err := store.WithLock(context.Background(), func() error {
		current, err := store.Load(context.Background())
		if err != nil {
			return err
		}
		time.Sleep(150 * time.Millisecond)
		switch field {
		case "title":
			current.Branding.PageTitle = "Concurrent title"
		case "view":
			current.DefaultView = "search"
		default:
			return fmt.Errorf("unknown helper field")
		}
		return store.Activate(context.Background(), current)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestConfigurationCrossProcessTransactionsAndReaders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(concurrentConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	store := configyaml.NewStore(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "test"})
	commands := make([]*exec.Cmd, 0, 2)
	for _, field := range []string{"title", "view"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestConfigurationConcurrentProcessHelper$")
		cmd.Env = append(os.Environ(), "TEST_CONFIGURATION_PATH="+path, "TEST_CONFIGURATION_FIELD="+field)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, cmd)
	}
	done := make(chan error, len(commands))
	for _, cmd := range commands {
		go func(cmd *exec.Cmd) { done <- cmd.Wait() }(cmd)
	}
	for remaining := len(commands); remaining > 0; {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
			remaining--
		case <-time.After(5 * time.Millisecond):
			if _, err := store.Load(context.Background()); err != nil {
				t.Fatalf("reader observed incomplete configuration: %v", err)
			}
		}
	}
	current, err := store.Load(context.Background())
	if err != nil || current.Branding.PageTitle != "Concurrent title" || current.DefaultView != "search" {
		t.Fatalf("lost update: %#v, %v", current, err)
	}
}

func TestConfigurationLockIsBoundedAndReleasedOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(concurrentConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	first := configyaml.NewStore(path, nil)
	second := configyaml.NewStore(path, nil)
	sentinel := errors.New("injected callback failure")
	err := first.WithLock(context.Background(), func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		if err := second.WithLock(ctx, func() error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("contended lock = %v, want deadline exceeded", err)
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("callback error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := second.WithLock(ctx, func() error { return nil }); err != nil {
		t.Fatalf("lock not released after failure: %v", err)
	}
}
