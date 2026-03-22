package app

import (
	"html/template"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type qrCodePublic struct {
	ID          int64
	Kind        string
	Title       string
	Description string
	ShowOnHome  bool
}

type qrPageData struct {
	Authors  []qrCodePublic
	Partners []qrCodePublic
}

func handleQRPage(pool *pgxpool.Pool, tpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(r.Context(), `
SELECT id, kind, title, description, show_on_home
FROM qr_codes
WHERE active=true
ORDER BY kind ASC, created_at ASC, id ASC`)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var authors, partners []qrCodePublic
		for rows.Next() {
			var q qrCodePublic
			if err := rows.Scan(&q.ID, &q.Kind, &q.Title, &q.Description, &q.ShowOnHome); err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if q.Kind == "author" {
				authors = append(authors, q)
			} else {
				partners = append(partners, q)
			}
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tpl.ExecuteTemplate(w, "qr.html", qrPageData{
			Authors:  authors,
			Partners: partners,
		})
	}
}
