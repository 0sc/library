package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/go-chi/chi"
	"go.uber.org/zap"
)

// contextKey
type key string

type service struct {
	logger *zap.Logger
	db     *bolt.DB
}

const (
	ratingIsInvalid   = "rating could not be parsed"
	ratingNotFoundErr = "rating not found"
	ratingFetchErr    = "could not load ratings"
	ratingSaveErr     = "rating could not be saved"

	rateableTypeParam = "rateableType"
	rateableKeyParam  = "rateableKey"
)

func newService(db *bolt.DB, logger *zap.Logger) *service {
	return &service{db: db, logger: logger}
}

func (svc *service) registerRoutes(r chi.Router) {
	// GET /authors/1234/ratings
	// POST /authors/1234/ratings

	pathWithParam := fmt.Sprintf("/{%s}/{%s}/ratings", rateableTypeParam, rateableKeyParam)
	r.With(svc.verifier).Route(pathWithParam, func(r chi.Router) {
		r.Get("/", svc.handleGet)
		r.Put("/", svc.handlePut)
	})

	r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK")
	})
}

func (svc *service) setup(cm []string) error {
	return setup(svc.db, cm)
}

func (svc *service) handlePut(w http.ResponseWriter, r *http.Request) {
	rt := &rating{}
	err := json.NewDecoder(r.Body).Decode(rt)
	if err != nil {
		svc.respondWithMsg(w, ratingIsInvalid, http.StatusBadRequest)
		svc.logger.Error(ratingIsInvalid, zap.Error(err))
		return
	}

	k := chi.URLParam(r, rateableKeyParam)
	rte := r.Context().Value(key(k)).(*rateable)

	rt, err = rte.save(*rt)
	if err != nil {
		svc.respondWithMsg(w, ratingSaveErr, http.StatusInternalServerError)
		svc.logger.Error(ratingSaveErr, zap.Error(err), zap.Any("rating", *rt))
		return
	}

	svc.respondWithPayload(w, rt, http.StatusOK)
}

func (svc *service) handleGet(w http.ResponseWriter, r *http.Request) {
	k := chi.URLParam(r, rateableKeyParam)
	rte := r.Context().Value(key(k)).(*rateable)

	rt, err := rte.get()
	if err != nil {
		svc.respondWithMsg(w, ratingFetchErr, http.StatusBadRequest)
		svc.logger.Error(
			ratingFetchErr,
			zap.Error(err),
			zap.String(rateableKeyParam, rte.key),
			zap.String(rateableTypeParam, rte.kind),
		)

		return
	}

	svc.respondWithPayload(w, rt, http.StatusOK)
}

func (svc *service) verifier(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		kind := chi.URLParam(r, rateableTypeParam)
		rKey := chi.URLParam(r, rateableKeyParam)

		if !verify(svc.db, kind) {
			svc.respondWithMsg(w, fmt.Sprintf(rateableTypeNotFoundFmt, kind), http.StatusNotAcceptable)
			svc.logger.Warn("could not verify rateable type", zap.String(rateableTypeParam, kind))
			return
		}

		rt := &rateable{db: svc.db, kind: kind, key: rKey}
		ctx := context.WithValue(r.Context(), key(rKey), rt)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

func (svc *service) respondWithMsg(w http.ResponseWriter, msg string, code int) {
	payload := struct {
		Message string `json:"message"`
	}{msg}

	svc.respondWithPayload(w, payload, code)
}

func (svc *service) respondWithPayload(w http.ResponseWriter, payload interface{}, code int) {
	data, err := json.Marshal(payload)
	if err != nil {
		code = http.StatusInternalServerError
		data = []byte(`{"message":"failed to prepare response. Please try again"}`)
	}

	svc.respond(w, data, code)
}

func (svc *service) respond(w http.ResponseWriter, data []byte, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}
