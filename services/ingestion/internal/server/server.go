package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func Run(ctx context.Context, h *Handlers, addr, password string) error {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler { return basicAuth(password, next) })

		r.Get("/", h.PastePage)
		r.Get("/api/novels", h.ListNovels)
		r.Post("/api/novels", h.CreateNovel)
		r.Get("/api/chapters", h.ListChapters)
		r.Post("/api/chapters", h.CreateChapter)
	})

	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("paste-to-db server listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: %w", err)
	}
	return nil
}

func basicAuth(password string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pwd, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pwd), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="paste-to-db"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
