package static

import (
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

type localFileSystem struct {
	http.FileSystem
	root    string
	indexes bool
}

func LocalFile(root string, indexes bool) *localFileSystem {
	return &localFileSystem{
		FileSystem: gin.Dir(root, indexes),
		root:       root,
		indexes:    indexes,
	}
}

func (l *localFileSystem) Exists(prefix string, file string) bool {
	if p := strings.TrimPrefix(file, prefix); len(p) < len(file) {
		name := path.Join(l.root, path.Clean(p))

		if strings.Contains(name, "\\") || strings.Contains(name, "..") {
			return false
		}

		stats, err := os.Stat(name)
		if err != nil {
			return false
		}
		if stats.IsDir() {
			if !l.indexes {
				_, err := os.Stat(path.Join(name, "index.html"))
				if err != nil {
					return false
				}
			}
		}
		return true
	}
	return false
}
