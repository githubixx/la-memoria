// Command bookmarker-config-init initializes or refreshes Compose configuration.
package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/githubixx/la-memoria/adapters/configyaml"
)

func main() {
	env := configyaml.Environment{}
	for _, entry := range os.Environ() {
		if name, value, ok := strings.Cut(entry, "="); ok {
			env[name] = value
		}
	}
	if err := configyaml.InitializeCompose(context.Background(), "/run/bookmarker/config.yaml", "/app/config.yaml.tmpl",
		os.Getenv("BOOKMARKER_CONFIG_IMPORT_PATH"), os.Getenv("BOOKMARKER_ADMIN_USERNAME"),
		os.Getenv("BOOKMARKER_ADMIN_PASSWORD_HASH"), env); err != nil {
		// Avoid echoing raw file contents, credentials, or environment values.
		slog.Error("configuration initialization failed; check credentials, volume permissions and the requested import, and remove the import setting after a successful import")
		os.Exit(1)
	}
}
