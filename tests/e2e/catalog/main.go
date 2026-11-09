// Command catalog exposes the pinned bundled IDs to browser tests only.
package main

import (
	"encoding/json"
	"os"

	chromaui "github.com/githubixx/chroma-ui"
)

func main() {
	catalog, err := chromaui.NewCatalog(nil)
	if err != nil {
		panic(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(catalog.Themes()); err != nil {
		panic(err)
	}
}
