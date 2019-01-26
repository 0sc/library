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

func Test_service_handlerPut(t *testing.T) {
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
			name:     "it does not add the rating if the payload is invalid",
			payload:  []byte(`{"five_stars": "4}`),
			path:     fmt.Sprintf("/%s/%s/ratings", kind, key),
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "it does not add the rating if resourceType does not exists",
			payload:  []byte(`{"five_stars": 4}`),
			path:     fmt.Sprintf("/unknownResourceType/%s/ratings", key),
			wantCode: http.StatusNotAcceptable,
		},
		{
			name:     "it creates resource and adds the rating if resource does not exist",
			payload:  []byte(`{"five_stars": 4}`),
			path:     fmt.Sprintf("/%s/another-key/ratings", kind),
			wantCode: http.StatusOK,
		},
		{
			name:     "it adds the rating to the resource if not empty",
			payload:  []byte(`{"five_stars": 4}`),
			path:     fmt.Sprintf("/%s/%s/ratings", kind, key),
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
			r := httptest.NewRequest(http.MethodPut, tt.path, body)

			mux.ServeHTTP(w, r)
			assert.Equal(t, tt.wantCode, w.Code)
		})
	}
}

func Test_service_handleGet(t *testing.T) {
	t.Parallel()

	db := setupDB()
	defer cleanup(db)

	kind := "posts"
	key := "my-key-1"
	rt := &rating{
		FiveStars:  1,
		FourStars:  2,
		ThreeStars: 3,
		TwoStars:   4,
		OneStars:   5,
	}
	var data []byte

	err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte(kind))
		if err != nil {
			return err
		}

		cb, err := b.CreateBucket([]byte(key))
		if err != nil {
			return err
		}

		data, err = json.Marshal(rt)
		if err != nil {
			return err
		}
		return cb.Put(ratingsKey, data)
	})
	assert.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		wantCode int
		want     string
	}{
		{
			name:     "it responds with error if rateableType does not exists",
			path:     fmt.Sprintf("/unknownResourceType/%s/ratings", key),
			want:     buildResp(fmt.Sprintf(rateableTypeNotFoundFmt, "unknownResourceType")),
			wantCode: http.StatusNotAcceptable,
		},
		{
			name:     "it responds with error if rating for resource with key does not exist",
			path:     fmt.Sprintf("/%s/another-key/ratings", kind),
			want:     buildResp(ratingFetchErr),
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "it responds with the rating",
			path:     fmt.Sprintf("/%s/%s/ratings", kind, key),
			want:     string(data),
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
			name:     "it returns error if it the rateable type does not exist",
			kind:     kind,
			wantBody: buildResp(fmt.Sprintf(rateableTypeNotFoundFmt, kind)),
		},
		{
			name: "it passes on the request if the rateable type exists",
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
			rctx.URLParams.Add(rateableTypeParam, tt.kind)
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
