package app

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func handlePartnerLogo(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var mime string
		var data []byte
		err := pool.QueryRow(r.Context(), `
SELECT partner_logo_mime, partner_logo_data
FROM app_settings
WHERE id = 1 AND partner_logo_data IS NOT NULL`).Scan(&mime, &data)
		if err != nil {
			if err == pgx.ErrNoRows {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if mime == "" {
			mime = "image/png"
		}
		w.Header().Set("Content-Type", mime)
		_, _ = w.Write(data)
	}
}

func handleQRImage(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			http.NotFound(w, r)
			return
		}
		var mime string
		var data []byte
		err = pool.QueryRow(r.Context(), `
SELECT image_mime, image_data
FROM qr_codes
WHERE id=$1 AND active=true`, id).Scan(&mime, &data)
		if err != nil {
			if err == pgx.ErrNoRows {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if mime == "" {
			mime = "image/png"
		}
		w.Header().Set("Content-Type", mime)
		_, _ = w.Write(data)
	}
}

