//go:build noembedlocales

package i18n

import "io/fs"

func getEmbeddedLocales() fs.FS {
	return nil
}
