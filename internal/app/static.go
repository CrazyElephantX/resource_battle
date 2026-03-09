package app

import (
	"io/fs"
	"net/http"

	"resource_battle/internal/web"
)

func staticHandler() (http.Handler, error) {
	sub, err := fs.Sub(web.FS, "static")
	if err != nil {
		return nil, err
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(sub))), nil
}

