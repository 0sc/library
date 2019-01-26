package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var buildResp = func(msg string) string {
	return fmt.Sprintf(`{"message":"%s"}`, msg)
}

func Test_service_handlerAdd(t *testing.T) {
	t.Parallel()

	kind := "posts"
	key := "my-key"
	tests := []struct {
		name     string
		path     string
		payload  []byte
		wantCode int
	}{
		{
			name:     "it does not add the comment to the resource if comment is empty",
			payload:  []byte(`{"value": ""}`),
			path:     fmt.Sprintf("/%s/%s/comments", kind, key),
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "it does not add the comment to payload is invalid",
			payload:  []byte(`{"value": "}`),
			path:     fmt.Sprintf("/%s/%s/comments", kind, key),
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "it does not add the comment if resourceType does not exists",
			payload:  []byte(`{"value": "my-coment"}`),
			path:     fmt.Sprintf("/unknownResourceType/%s/comments", key),
			wantCode: http.StatusNotAcceptable,
		},
		{
			name:     "it creates resource and adds the comment if resource does not exist",
			payload:  []byte(`{"value": "my-coment"}`),
			path:     fmt.Sprintf("/%s/another-key/comments", kind),
			wantCode: http.StatusOK,
		},
		{
			name:     "it adds the comment to resource if not empty",
			payload:  []byte(`{"value": "my-coment"}`),
			path:     fmt.Sprintf("/%s/%s/comments", kind, key),
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupDB()
			defer cleanup(db)

			err := db.Update(func(tx *bolt.Tx) error {
				b, err := tx.CreateBucket([]byte(kind))
				if err != nil {
					return err
				}

				_, err = b.CreateBucket([]byte(key))
				return err
			})
			assert.NoError(t, err)

			mux := chi.NewRouter()
			svc := newService(db, zap.NewNop())
			svc.registerRoutes(mux)

			w := httptest.NewRecorder()
			body := bytes.NewBuffer(tt.payload)
			r := httptest.NewRequest(http.MethodPost, tt.path, body)

			mux.ServeHTTP(w, r)
			assert.Equal(t, tt.wantCode, w.Code)
		})
	}
}

func Test_service_handleList(t *testing.T) {
	t.Parallel()

	db := setupDB()
	defer cleanup(db)

	kind := "posts"
	keyOne := "my-key-1"
	keyTwo := "my-key-2"

	err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte(kind))
		if err != nil {
			return err
		}

		_, err = b.CreateBucket([]byte(keyOne))
		if err != nil {
			return err
		}

		_, err = b.CreateBucket([]byte(keyTwo))
		return err
	})
	assert.NoError(t, err)

	cm := &commentable{db: db, key: keyOne, kind: kind}
	commentOne, err := cm.add(&comment{Value: "foo"})
	assert.NoError(t, err)
	commentTwo, err := cm.add(&comment{Value: "bar"})
	assert.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		wantCode int
		wantBody string
	}{
		{
			name:     "it returns all the comment for the resource with the given key",
			path:     fmt.Sprintf("/%s/%s/comments", kind, keyOne),
			wantCode: http.StatusOK,
			wantBody: fmt.Sprintf(
				`{"comments":[{"id":"%s","value":"%s"},{"id":"%s","value":"%s"}]}`, commentOne.ID, commentOne.Value,
				commentTwo.ID, commentTwo.Value),
		},
		{
			name:     "it returns empty if no comment exists for the resource with the given key",
			path:     fmt.Sprintf("/%s/%s/comments", kind, keyTwo),
			wantBody: `{"comments":[]}`,
			wantCode: http.StatusOK,
		},
		{
			name:     "it returns error if resource with key not found",
			path:     fmt.Sprintf("/%s/my-key-3/comments", kind),
			wantBody: buildResp(fmt.Sprintf(commentableNotFoundFmt, kind, "my-key-3")),
			wantCode: http.StatusNotFound,
		},
		{
			name:     "it returns error if resource type does not exist",
			path:     fmt.Sprintf("/unknownResource/%s/comments", keyTwo),
			wantBody: buildResp(fmt.Sprintf(commentableTypeNotFoundFmt, "unknownResource")),
			wantCode: http.StatusNotAcceptable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := chi.NewRouter()
			svc := newService(db, zap.NewNop())
			svc.registerRoutes(mux)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)

			mux.ServeHTTP(w, r)
			assert.Equal(t, tt.wantCode, w.Code)
			assert.Equal(t, tt.wantBody, w.Body.String())
		})
	}
}

func Test_service_handleGet(t *testing.T) {
	t.Parallel()

	db := setupDB()
	defer cleanup(db)

	kind := "posts"
	key := "my-key-1"
	cmt := &comment{ID: "12345", Value: "something"}

	err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte(kind))
		if err != nil {
			return err
		}

		cb, err := b.CreateBucket([]byte(key))
		if err != nil {
			return err
		}

		ccb, err := cb.CreateBucket([]byte("comments"))
		if err != nil {
			return err
		}

		data, err := json.Marshal(cmt)
		if err != nil {
			return err
		}
		return ccb.Put([]byte(cmt.ID), data)
	})
	assert.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		wantCode int
		want     string
	}{
		{
			name:     "it responds with error if resourceType does not exists",
			path:     fmt.Sprintf("/unknownResourceType/%s/comments/%s", key, cmt.ID),
			want:     buildResp(fmt.Sprintf(commentableTypeNotFoundFmt, "unknownResourceType")),
			wantCode: http.StatusNotAcceptable,
		},
		{
			name:     "it responds with error if resource with id does not exist",
			path:     fmt.Sprintf("/%s/another-key/comments/%s", kind, cmt.ID),
			want:     buildResp(fmt.Sprintf(commentableNotFoundFmt, kind, "another-key")),
			wantCode: http.StatusNotFound,
		},
		{
			name:     "it responds with error if comment for resource with comment id does not exist",
			path:     fmt.Sprintf("/%s/%s/comments/another-key", kind, key),
			want:     buildResp(commentNotFoundErr),
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "it responds with the comment",
			path:     fmt.Sprintf("/%s/%s/comments/%s", kind, key, cmt.ID),
			want:     fmt.Sprintf(`{"id":"%s","value":"%s"}`, cmt.ID, cmt.Value),
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := chi.NewRouter()
			svc := newService(db, zap.NewNop())
			svc.registerRoutes(mux)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)

			mux.ServeHTTP(w, r)

			assert.Equal(t, tt.wantCode, w.Code)
			assert.Equal(t, tt.want, w.Body.String())
		})
	}
}

func Test_service_handleRemove(t *testing.T) {
	t.Parallel()

	db := setupDB()
	defer cleanup(db)

	kind := "posts"
	key := "my-key-1"
	cmt := &comment{ID: "12345", Value: "something"}

	err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte(kind))
		if err != nil {
			return err
		}

		cb, err := b.CreateBucket([]byte(key))
		if err != nil {
			return err
		}

		ccb, err := cb.CreateBucket([]byte("comments"))
		if err != nil {
			return err
		}

		data, err := json.Marshal(cmt)
		if err != nil {
			return err
		}
		return ccb.Put([]byte(cmt.ID), data)
	})
	assert.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		wantCode int
		want     string
	}{
		{
			name:     "it responds with error if resourceType does not exists",
			path:     fmt.Sprintf("/unknownResourceType/%s/comments/%s", key, cmt.ID),
			want:     buildResp(fmt.Sprintf(commentableTypeNotFoundFmt, "unknownResourceType")),
			wantCode: http.StatusNotAcceptable,
		},
		{
			name:     "it responds with error if resource with id does not exist",
			path:     fmt.Sprintf("/%s/another-key/comments/%s", kind, cmt.ID),
			want:     buildResp(fmt.Sprintf(commentableNotFoundFmt, kind, "another-key")),
			wantCode: http.StatusNotFound,
		},
		{
			name:     "it responds with error if comment for resource with comment id does not exist",
			path:     fmt.Sprintf("/%s/%s/comments/another-key", kind, key),
			want:     buildResp(commentNotFoundErr),
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "it removes the comment and responds with success",
			path:     fmt.Sprintf("/%s/%s/comments/%s", kind, key, cmt.ID),
			want:     fmt.Sprintf(`{"message":"successfully deleted %s comment with id: %s"}`, kind, cmt.ID),
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := chi.NewRouter()
			svc := newService(db, zap.NewNop())
			svc.registerRoutes(mux)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodDelete, tt.path, nil)

			mux.ServeHTTP(w, r)

			assert.Equal(t, tt.wantCode, w.Code)
			assert.Equal(t, tt.want, w.Body.String())
		})
	}
}

func Test_service_handleUpdate(t *testing.T) {
	t.Parallel()

	db := setupDB()
	defer cleanup(db)

	kind := "posts"
	key := "my-key-1"
	cmt := &comment{ID: "12345", Value: "something"}

	err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte(kind))
		if err != nil {
			return err
		}

		cb, err := b.CreateBucket([]byte(key))
		if err != nil {
			return err
		}

		ccb, err := cb.CreateBucket([]byte("comments"))
		if err != nil {
			return err
		}

		data, err := json.Marshal(cmt)
		if err != nil {
			return err
		}
		return ccb.Put([]byte(cmt.ID), data)
	})
	assert.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		payload  []byte
		wantCode int
		want     string
	}{
		{
			name:     "it does not update the resource comment if comment is empty",
			payload:  []byte(`{"value": ""}`),
			path:     fmt.Sprintf("/%s/%s/comments/%s", kind, key, cmt.ID),
			want:     buildResp(commentIsInvalid),
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "it does not add the comment to payload is invalid",
			payload:  []byte(`{"value": "}`),
			path:     fmt.Sprintf("/%s/%s/comments/%s", kind, key, cmt.ID),
			want:     buildResp(commentIsInvalid),
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "it does not add the comment if resourceType does not exists",
			payload:  []byte(`{"value": "my-coment"}`),
			path:     fmt.Sprintf("/unknownResourceType/%s/comments/%s", key, cmt.ID),
			want:     buildResp(fmt.Sprintf(commentableTypeNotFoundFmt, "unknownResourceType")),
			wantCode: http.StatusNotAcceptable,
		},
		{
			name:     "it returns error if resource with id does not exist",
			payload:  []byte(`{"value": "my-coment"}`),
			path:     fmt.Sprintf("/%s/another-key/comments/%s", kind, cmt.ID),
			want:     buildResp(fmt.Sprintf(commentableNotFoundFmt, kind, "another-key")),
			wantCode: http.StatusNotFound,
		},
		{
			name:     "it returns error if comment for resource with comment id does not exist",
			payload:  []byte(`{"value": "my-coment"}`),
			path:     fmt.Sprintf("/%s/%s/comments/another-key", kind, key),
			want:     buildResp(commentNotFoundErr),
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "it updates the comment",
			payload:  []byte(`{"value": "my new comment"}`),
			path:     fmt.Sprintf("/%s/%s/comments/%s", kind, key, cmt.ID),
			want:     fmt.Sprintf(`{"id":"%s","value":"my new comment"}`, cmt.ID),
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := chi.NewRouter()
			svc := newService(db, zap.NewNop())
			svc.registerRoutes(mux)

			w := httptest.NewRecorder()
			body := bytes.NewBuffer(tt.payload)
			r := httptest.NewRequest(http.MethodPatch, tt.path, body)

			mux.ServeHTTP(w, r)

			assert.Equal(t, tt.wantCode, w.Code)
			assert.Equal(t, tt.want, w.Body.String())
		})
	}
}

func Test_servicer_verifier(t *testing.T) {
	t.Parallel()

	kind := "posts"
	tests := []struct {
		name      string
		setupFunc func(*bolt.Tx) error
		kind      string
		wantBody  string
		pass      bool
	}{
		{
			name:     "it returns error if it the resource type does not exist",
			kind:     kind,
			wantBody: buildResp(fmt.Sprintf(commentableTypeNotFoundFmt, kind)),
		},
		{
			name: "it passes on the request if the resource already exists",
			kind: kind,
			setupFunc: func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte(kind))
				return err
			},
			pass: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupDB()
			defer cleanup(db)

			if tt.setupFunc != nil {
				assert.NoError(t, db.Update(tt.setupFunc))
			}

			svc := &service{logger: zap.NewNop(), db: db}

			var passed bool
			fn := func(w http.ResponseWriter, r *http.Request) {
				passed = true
			}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add(commentableTypeParam, tt.kind)
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler := svc.verifier(http.HandlerFunc(fn))
			handler.ServeHTTP(w, r)

			assert.Equal(t, tt.pass, passed)
			assert.Equal(t, tt.wantBody, w.Body.String())
		})
	}
}

func Test_service_creator(t *testing.T) {
	t.Parallel()

	kind := "posts"
	key := "my-key"
	tests := []struct {
		name      string
		setupFunc func(*bolt.Tx) error
		key       string
		kind      string
		wantBody  string
		pass      bool
	}{
		{
			name:     "it returns error if it the resource type does not exist",
			kind:     kind,
			wantBody: buildResp(commentableSaveErr),
		},
		{
			name: "it returns error if it can't create the resource",
			kind: kind,
			setupFunc: func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte(kind))
				return err
			},
			wantBody: buildResp(commentableSaveErr),
		},
		{
			name: "it passes on the request if resources is created successfully",
			kind: kind,
			setupFunc: func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte(kind))
				return err
			},
			key:  key,
			pass: true,
		},
		{
			name: "it creates resource and passes on the request if the resource type already exists",
			kind: kind,
			key:  key,
			setupFunc: func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte(kind))
				return err
			},
			pass: true,
		},
		{
			name: "it passes on the request if the resource already exists",
			kind: kind,
			key:  key,
			setupFunc: func(tx *bolt.Tx) error {
				b, err := tx.CreateBucket([]byte(kind))
				if err != nil {
					return err
				}

				_, err = b.CreateBucket([]byte(key))
				return err
			},
			pass: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupDB()
			defer cleanup(db)

			if tt.setupFunc != nil {
				assert.NoError(t, db.Update(tt.setupFunc))
			}

			svc := &service{logger: zap.NewNop(), db: db}

			var passed bool
			fn := func(w http.ResponseWriter, r *http.Request) {
				passed = true
			}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add(commentableTypeParam, tt.kind)
			rctx.URLParams.Add(commentableKeyParam, tt.key)
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler := svc.creator(http.HandlerFunc(fn))
			handler.ServeHTTP(w, r)

			assert.Equal(t, tt.pass, passed)
			assert.Equal(t, tt.wantBody, w.Body.String())
		})
	}
}

func Test_service_validator(t *testing.T) {
	t.Parallel()

	key := "my-key"
	kind := "resource"
	errMsg := buildResp(fmt.Sprintf(commentableNotFoundFmt, kind, key))
	tests := []struct {
		name      string
		setupFunc func(*bolt.Tx) error
		wantBody  string
		pass      bool
	}{
		{
			name:     "it returns error if resource type does not exist",
			wantBody: errMsg,
		},
		{
			name: "it returns error if resource does not exist",
			setupFunc: func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte(kind))
				return err
			},
			wantBody: errMsg,
		},
		{
			name: "it passes on the request if the resource exists",
			setupFunc: func(tx *bolt.Tx) error {
				b, err := tx.CreateBucket([]byte(kind))
				if err != nil {
					return err
				}

				_, err = b.CreateBucket([]byte(key))
				return err
			},
			pass: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupDB()
			defer cleanup(db)

			if tt.setupFunc != nil {
				assert.NoError(t, db.Update(tt.setupFunc))
			}

			svc := &service{logger: zap.NewNop(), db: db}

			var passed bool
			fn := func(w http.ResponseWriter, r *http.Request) {
				passed = true
			}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add(commentableTypeParam, kind)
			rctx.URLParams.Add(commentableKeyParam, key)
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler := svc.validator(http.HandlerFunc(fn))
			handler.ServeHTTP(w, r)

			assert.Equal(t, tt.pass, passed)
			assert.Equal(t, tt.wantBody, w.Body.String())
		})
	}
}

func Test_respondWithMsg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  string
		code int
		want string
	}{
		{
			name: "it sets the response to the given message and status code - 1",
			msg:  "hello",
			code: http.StatusOK,
			want: `{"message":"hello"}`,
		},
		{
			name: "it sets the response to the given message and status code - 2",
			msg:  "wo%rk",
			code: http.StatusInternalServerError,
			want: `{"message":"wo%rk"}`,
		},
		{
			name: "it sets the response to the given message and status code - 3",
			msg:  "attempt was successful",
			code: http.StatusMovedPermanently,
			want: `{"message":"attempt was successful"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			svc := &service{}
			svc.respondWithMsg(w, tt.msg, tt.code)

			assert.Equal(t, tt.code, w.Code)
			assert.Equal(t, tt.want, w.Body.String())
		})
	}
}

func Test_respondWithPayload(t *testing.T) {
	t.Parallel()

	code := http.StatusOK
	tests := []struct {
		name     string
		payload  interface{}
		wantBody string
		wantCode int
	}{
		{
			name:     "it sends an error msg if it fails to marshal payload",
			payload:  math.Inf(1),
			wantBody: `{"message":"failed to prepare response. Please try again"}`,
			wantCode: http.StatusInternalServerError,
		},
		{
			name:     "it sends the payload as response body",
			payload:  struct{ Hello string }{"World"},
			wantBody: `{"Hello":"World"}`,
			wantCode: code,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			svc := &service{}
			svc.respondWithPayload(w, tt.payload, code)

			assert.Equal(t, tt.wantCode, w.Code)
			assert.Equal(t, tt.wantBody, w.Body.String())
		})
	}
}
