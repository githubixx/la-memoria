// Command bookmarker-cli exposes Bookmarker use cases through the
// versioned JSON Lines protocol on stdin/stdout, with diagnostics on
// stderr only.
package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/githubixx/la-memoria/adapters/agentbrowser"
	"github.com/githubixx/la-memoria/adapters/configyaml"
	"github.com/githubixx/la-memoria/adapters/filesystem"
	"github.com/githubixx/la-memoria/adapters/jsonl"
	"github.com/githubixx/la-memoria/adapters/observability"
	"github.com/githubixx/la-memoria/adapters/postgres"
	"github.com/githubixx/la-memoria/bookmarker/ports"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
)

func main() {
	logger := observability.NewLogger(os.Stderr)

	configPath := os.Getenv("BOOKMARKER_CONFIG")
	if configPath == "" {
		configPath = "config.yaml"
	}
	configuration, err := configyaml.Load(configPath, environmentValues())
	if err != nil {
		logger.Error("load configuration", "error", err.Error())
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, postgres.DSN(configuration))
	if err != nil {
		logger.Error("connect PostgreSQL", "error", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	if os.Getenv("BOOKMARKER_ALLOW_MIGRATIONS") == "true" {
		version, err := postgres.RunMigrations(ctx, pool, "migrations")
		if err != nil {
			logger.Error("run migrations", "error", err.Error())
			os.Exit(1)
		}
		logger.Info("schema ready", "version", version)
	}

	authStore, err := postgres.NewAuthStore(ctx, postgres.DSN(configuration))
	if err != nil {
		logger.Error("open auth store", "error", err.Error())
		os.Exit(1)
	}
	defer authStore.Close()

	screenshots, err := filesystem.NewScreenshotStore(configuration.Screenshots.Root)
	if err != nil {
		logger.Error("open screenshot store", "error", err.Error())
		os.Exit(1)
	}
	captureDrafts := postgres.NewCaptureDraftStore(pool)
	bookmarkQueryStore := postgres.NewBookmarkQueryStore(pool)
	bookmarkCreateStore := postgres.NewBookmarkCreateStore(pool)
	bookmarkMaintainStore := postgres.NewBookmarkMaintainStore(pool)
	retentionStore := postgres.NewRetentionStore(pool)
	retentionProcessor := usecase.NewRetentionProcessor(usecase.RetentionDependencies{Store: retentionStore, CleanupStore: retentionStore, Files: screenshots})
	if err := retentionProcessor.Process(ctx); err != nil {
		logger.Error("process retention", "error", err.Error())
	}

	configurationStore := configyaml.NewStore(configPath, environmentValues())
	server := jsonl.NewServer(jsonl.Dependencies{
		Browser:  usecase.NewBrowser(usecase.BrowseDependencies{Bookmarks: bookmarkQueryStore, PageSize: 10}),
		Searcher: usecase.NewSearcher(usecase.SearchDependencies{Bookmarks: bookmarkQueryStore, PageSize: configuration.Search.PageSize, MaximumResults: configuration.Search.MaximumResults}),
		Authenticator: usecase.NewAuthenticator(usecase.AuthenticationDependencies{
			Sessions:       authStore,
			LoginThrottles: authStore,
			Administrator:  configuration.Administrator,
		}),
		CaptureService: usecase.NewCaptureService(usecase.CaptureDependencies{
			Capturer: agentbrowser.NewCapturer(agentbrowser.Dependencies{Runner: processRunner{}}),
			Files:    screenshots,
			Drafts:   captureDrafts,
		}),
		Creator: usecase.NewCreator(usecase.CreateDependencies{
			Bookmarks: bookmarkCreateStore,
			Drafts:    captureDrafts,
			Files:     screenshots,
		}),
		Maintainer: usecase.NewMaintainer(usecase.MaintainDependencies{
			Bookmarks: bookmarkMaintainStore,
			Drafts:    captureDrafts,
			Files:     screenshots,
		}),
		Configuration: usecase.NewConfigurationService(usecase.ConfigurationDependencies{
			Store: configurationStore,
			Validator: usecase.ConfigurationValidators{
				configurationStore,
				filesystem.NewConfigurationValidator(staticAssetRoot()),
				postgres.NewConfigurationValidator(),
			},
			Activator: configurationStore,
		}),
	})
	if err := server.Serve(os.Stdin, os.Stdout, os.Stderr); err != nil {
		logger.Error("serve JSON Lines", "error", err.Error())
		os.Exit(1)
	}
}

func staticAssetRoot() string {
	root, err := filepath.Abs("web/static")
	if err != nil {
		return "web/static"
	}
	return root
}

func environmentValues() configyaml.Environment {
	values := configyaml.Environment{}
	for _, entry := range os.Environ() {
		for index := 0; index < len(entry); index++ {
			if entry[index] == '=' {
				values[entry[:index]] = entry[index+1:]
				break
			}
		}
	}
	return values
}

// processRunner invokes the external agent-browser executable found on PATH.
type processRunner struct{}

func (processRunner) Run(ctx context.Context, arguments ...string) (ports.ProcessResult, error) {
	command := exec.CommandContext(ctx, "agent-browser", arguments...)
	var stdout, stderr []byte
	stdoutPipe, err := command.StdoutPipe()
	if err != nil {
		return ports.ProcessResult{}, err
	}
	stderrPipe, err := command.StderrPipe()
	if err != nil {
		return ports.ProcessResult{}, err
	}
	if err := command.Start(); err != nil {
		return ports.ProcessResult{}, err
	}
	stdout, _ = readAll(stdoutPipe)
	stderr, _ = readAll(stderrPipe)
	err = command.Wait()
	return ports.ProcessResult{Stdout: string(stdout), Stderr: string(stderr)}, err
}

func readAll(reader interface{ Read([]byte) (int, error) }) ([]byte, error) {
	buffer := make([]byte, 0, 4096)
	chunk := make([]byte, 4096)
	for {
		n, err := reader.Read(chunk)
		buffer = append(buffer, chunk[:n]...)
		if err != nil {
			return buffer, nil
		}
	}
}
