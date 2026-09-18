module github.com/arkiifr/afferr

go 1.23.0

require (
	github.com/charmbracelet/bubbles v0.18.0
	github.com/charmbracelet/bubbletea v1.2.4
	github.com/charmbracelet/lipgloss v0.13.0
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
	github.com/spf13/cobra v1.8.1
	golang.org/x/sync v0.8.0
	modernc.org/sqlite v1.33.1
	github.com/muhammadmuzzammil1998/jsonc v0.0.0
)

replace github.com/muhammadmuzzammil1998/jsonc => ./third_party/jsonc
