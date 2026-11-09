package ports

// ThemeOption is safe metadata for a bundled, selectable theme.
type ThemeOption struct {
	ID      string
	Name    string
	Bundled bool
}

type ThemeCatalog interface {
	Themes() []ThemeOption
	Contains(string) bool
}
