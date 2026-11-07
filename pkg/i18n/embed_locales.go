//go:build !noembedlocales

package i18n

import (
	"embed"
	"io/fs"
)

//go:embed locales/*.arb
var embeddedLocalesFS embed.FS

func getEmbeddedLocales() fs.FS {
	return embeddedLocalesFS
}
