package web

import (
	"embed"
	"html/template"
	"time"
)

//go:embed templates/*.html static/*
var FS embed.FS

func MustTemplates() *template.Template {
	funcs := template.FuncMap{
		"checked": func(m map[int64]map[int64]bool, teamID int64, taskID int64) bool {
			if m == nil {
				return false
			}
			mt, ok := m[teamID]
			if !ok || mt == nil {
				return false
			}
			return mt[taskID]
		},
		"formatTime": func(t *time.Time) string {
			if t == nil {
				return "—"
			}
			return t.Local().Format("2006-01-02 15:04")
		},
		"derefInt64": func(p *int64) int64 {
			if p == nil {
				return 0
			}
			return *p
		},
	}

	t, err := template.New("root").Funcs(funcs).ParseFS(FS, "templates/*.html")
	if err != nil {
		panic(err)
	}
	return t
}
