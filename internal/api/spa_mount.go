package api

import (
	"embed"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// Embedded UI assets.
//
// The build process copies the Angular production build output into
// internal/api/dist/ before compiling Go.
//
// Example layout after build:
// internal/api/dist/
//
//	index.html
//	assets/
//	  ...
//	*.js, *.css
//
//go:embed dist/browser/*
var embeddedUI embed.FS

func getEmbedFs() static.ServeFileSystem {
	fs, err := static.EmbedFolder(embeddedUI, "dist/browser")
	if err != nil {
		panic("failed to get embedded UI filesystem: " + err.Error())
	}
	return fs
}

// MountSPA mounts either the embedded Angular app (if built) or a placeholder.
// No build flags required—if internal/api/dist/ is empty, it gracefully serves a placeholder.
func MountSPA(r *gin.Engine, logger *slog.Logger) {
	distFS := getEmbedFs()
	r.Use(static.Serve("/", distFS))

	r.NoRoute(func(c *gin.Context) {
		// Only serve index.html for non-API routes
		if !strings.HasPrefix(c.Request.RequestURI, "/api") {
			index, err := distFS.Open("index.html")
			if err != nil {
				logger.Error("failed to open index.html", "error", err)
			}
			defer index.Close()
			stat, _ := index.Stat()
			http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), index)
		}
	})
}
