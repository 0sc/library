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
	commentIsInvalid   = "comment could not be parsed"
	commentNotFoundErr = "comment not found"
	commentListErr     = "could not load comments"
	commentDeleteErr   = "comment could not be deleted"
	commentSaveErr     = "comment could not be saved"
	commentableSaveErr = "could not provision comments"

	commentableTypeParam = "commentableType"
	commentableKeyParam  = "commentableKey"
	commentKeyParam      = "commentKey"
)

func newService(db *bolt.DB, logger *zap.Logger) *service {
	return &service{db: db, logger: logger}
}

func (svc *service) registerRoutes(r chi.Router) {
	r.With(svc.verifier).Route(fmt.Sprintf("/{%s}", commentableTypeParam), func(r chi.Router) {
		// create resource comment bucket if not exists
		// validate resourceKey
		r.With(svc.creator, svc.validator).
			Post(fmt.Sprintf("/{%s}/comments", commentableKeyParam), svc.handleAdd)

		// validate resourceKey
		pathWithParam := fmt.Sprintf("/comments/{%s}", commentKeyParam)
		r.With(svc.validator).Route(fmt.Sprintf("/{%s}", commentableKeyParam), func(r chi.Router) {
			r.Get("/comments", svc.handleList)
			r.Get(pathWithParam, svc.handleGet)
			r.Delete(pathWithParam, svc.handleRemove)
			r.Patch(pathWithParam, svc.handleUpdate)
		})
	})

	r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK")
	})
}

func (svc *service) setup(cm []string) error {
	return setup(svc.db, cm)
}

func (svc *service) handleAdd(w http.ResponseWriter, r *http.Request) {
	co := &comment{}
	err := json.NewDecoder(r.Body).Decode(co)
	if err != nil || co.Value == "" {
		svc.respondWithMsg(w, commentIsInvalid, http.StatusBadRequest)
		svc.logger.Error(commentIsInvalid, zap.Error(err))
		return
	}

	k := chi.URLParam(r, commentableKeyParam)
	c := r.Context().Value(key(k)).(*commentable)

	co, err = c.add(co)
	if err != nil {
		svc.respondWithMsg(w, commentSaveErr, http.StatusInternalServerError)
		svc.logger.Error(commentSaveErr, zap.Error(err), zap.String("comment", co.Value))
		return
	}

	svc.respondWithPayload(w, co, http.StatusOK)
}

func (svc *service) handleUpdate(w http.ResponseWriter, r *http.Request) {
	co := &comment{}
	err := json.NewDecoder(r.Body).Decode(co)
	if err != nil || co.Value == "" {
		svc.respondWithMsg(w, commentIsInvalid, http.StatusBadRequest)
		svc.logger.Error(commentIsInvalid, zap.Error(err))
		return
	}

	k := chi.URLParam(r, commentableKeyParam)
	c := r.Context().Value(key(k)).(*commentable)
	cKey := chi.URLParam(r, commentKeyParam)
	l := svc.logger.With(
		zap.String(commentKeyParam, cKey),
		zap.String(commentableKeyParam, c.key),
		zap.String(commentableTypeParam, c.kind),
	)
	cmt, err := c.get(cKey)
	if err != nil {
		svc.respondWithMsg(w, commentNotFoundErr, http.StatusBadRequest)
		l.Error(commentNotFoundErr, zap.Error(err))
		return
	}

	cmt.Value = co.Value
	cmt, err = c.save(cmt)
	if err != nil {
		svc.respondWithMsg(w, commentSaveErr, http.StatusInternalServerError)
		l.Error(commentSaveErr, zap.Error(err), zap.String("comment", cmt.Value))
		return
	}

	svc.respondWithPayload(w, cmt, http.StatusOK)
}

func (svc *service) handleList(w http.ResponseWriter, r *http.Request) {
	k := chi.URLParam(r, commentableKeyParam)
	c := r.Context().Value(key(k)).(*commentable)

	var data struct {
		Comments []*comment `json:"comments"`
	}
	var err error
	data.Comments, err = c.list()
	if err != nil {
		svc.respondWithMsg(w, fmt.Sprintf("error fetching comments: %v", err), http.StatusInternalServerError)
		svc.logger.Error(
			commentListErr,
			zap.Error(err),
			zap.String(commentableKeyParam, c.key),
			zap.String(commentableTypeParam, c.kind),
		)
	}

	svc.respondWithPayload(w, data, http.StatusOK)
}

func (svc *service) handleGet(w http.ResponseWriter, r *http.Request) {
	k := chi.URLParam(r, commentableKeyParam)
	c := r.Context().Value(key(k)).(*commentable)
	cKey := chi.URLParam(r, commentKeyParam)
	cmt, err := c.get(cKey)
	if err != nil {
		svc.respondWithMsg(w, commentNotFoundErr, http.StatusBadRequest)
		svc.logger.Error(
			commentNotFoundErr,
			zap.Error(err),
			zap.String(commentKeyParam, cKey),
			zap.String(commentableKeyParam, c.key),
			zap.String(commentableTypeParam, c.kind),
		)
		return
	}

	svc.respondWithPayload(w, cmt, http.StatusOK)
}

func (svc *service) handleRemove(w http.ResponseWriter, r *http.Request) {
	k := chi.URLParam(r, commentableKeyParam)
	c := r.Context().Value(key(k)).(*commentable)
	cKey := chi.URLParam(r, commentKeyParam)
	l := svc.logger.With(
		zap.String(commentKeyParam, cKey),
		zap.String(commentableKeyParam, c.key),
		zap.String(commentableTypeParam, c.kind),
	)

	cmt, err := c.get(cKey)
	if err != nil {
		svc.respondWithMsg(w, commentNotFoundErr, http.StatusBadRequest)
		l.Error(commentNotFoundErr, zap.Error(err))
		return
	}

	err = c.remove(cmt.ID)
	if err != nil {
		svc.respondWithMsg(w, commentDeleteErr, http.StatusInternalServerError)
		l.Error(commentDeleteErr, zap.Error(err))
		return
	}

	svc.respondWithMsg(w, fmt.Sprintf("successfully deleted %s comment with id: %s", c.kind, cmt.ID), http.StatusOK)
}

// validator validates that a resource of the given key exists for the given resource kind
func (svc *service) validator(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		cKind := chi.URLParam(r, commentableTypeParam)
		cKey := chi.URLParam(r, commentableKeyParam)

		c := &commentable{db: svc.db, key: cKey, kind: cKind}
		if !c.exists() {
			svc.respondWithMsg(w, fmt.Sprintf(commentableNotFoundFmt, c.kind, c.key), http.StatusNotFound)
			svc.logger.Warn("commentable validation failed",
				zap.String(commentableKeyParam, cKey),
				zap.String(commentableTypeParam, cKind))
			return
		}

		ctx := context.WithValue(r.Context(), key(cKey), c)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

// creator creates a new resource with the given key of the given resource kind if not exists
// it should be used by the create comment action to enable creating new resources when add comment rquests are sent
func (svc *service) creator(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		cKind := chi.URLParam(r, commentableTypeParam)
		cKey := chi.URLParam(r, commentableKeyParam)

		c := &commentable{kind: cKind, key: cKey, db: svc.db}
		err := c.ensure()
		if err != nil {
			svc.respondWithMsg(w, commentableSaveErr, http.StatusNotAcceptable)
			svc.logger.Error(commentableSaveErr,
				zap.String(commentableKeyParam, cKey),
				zap.String(commentableTypeParam, cKind))
			return
		}

		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func (svc *service) verifier(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		kind := chi.URLParam(r, commentableTypeParam)

		if !verify(svc.db, kind) {
			svc.respondWithMsg(w, fmt.Sprintf(commentableTypeNotFoundFmt, kind), http.StatusNotAcceptable)
			svc.logger.Warn(commentableSaveErr, zap.String(commentableTypeParam, kind))
			return
		}

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
