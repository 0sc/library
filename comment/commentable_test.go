package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/stretchr/testify/assert"
)

func tempfile() string {
	f, err := ioutil.TempFile("", "boltdb-")
	if err != nil {
		panic(err)
	}
	if err := f.Close(); err != nil {
		panic(err)
	}
	if err := os.Remove(f.Name()); err != nil {
		panic(err)
	}
	return f.Name()
}

func setupDB() *bolt.DB {
	path := tempfile()
	db, err := bolt.Open(path, 0666, nil)
	if err != nil {
		panic(err)
	}

	return db
}

func cleanup(db *bolt.DB) {
	// close db and remove file
	defer os.Remove(db.Path())
	if err := db.Close(); err != nil {
		panic(err)
	}
}

func Test_commentable_ensure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		resourceType string
		resourceKey  string
		setupFunc    func(*bolt.Tx) error
		wantErr      error
	}{
		{
			name:         "it returns error if resourceType doesn not exist",
			resourceType: "resource",
			wantErr:      fmt.Errorf("resource 'resource' does not exist"),
		},
		{
			name:         "it returns error if create resource bucket fails",
			resourceType: "resource",
			setupFunc: func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte("resource"))
				return err
			},
			wantErr: bolt.ErrBucketNameRequired,
		},
		{
			name:         "it creates resource type if not exists",
			resourceType: "resource",
			resourceKey:  "resourceID",
			setupFunc: func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte("resource"))
				return err
			},
		},
		{
			name: "it returns with no errors if resource already exists",
			setupFunc: func(tx *bolt.Tx) error {
				b, err := tx.CreateBucket([]byte("resource"))
				if err != nil {
					return err
				}

				_, err = b.CreateBucket([]byte("resourceID"))
				return err
			},
			resourceType: "resource",
			resourceKey:  "resourceID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupDB()
			defer cleanup(db)

			if tt.setupFunc != nil {
				assert.NoError(t, db.Update(tt.setupFunc))
			}

			cc := &commentable{db: db, key: tt.resourceKey, kind: tt.resourceType}
			err := cc.ensure()
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func Test_commentable_exists(t *testing.T) {
	t.Parallel()

	kind := "resource"
	key := "resourceID"
	tests := []struct {
		name      string
		setupFunc func(*bolt.Tx) error
		want      bool
	}{
		{
			name: "it returns true if resource type with key exists",
			setupFunc: func(tx *bolt.Tx) error {
				b, err := tx.CreateBucket([]byte(kind))
				if err != nil {
					return err
				}

				_, err = b.CreateBucket([]byte(key))
				return err
			},
			want: true,
		},
		{
			name: "it returns false if resource type does not exist",
			setupFunc: func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte("some-other-kind"))
				return err
			},
		},
		{
			name: "it returns false if resource with key does not exist for the resource type",
			setupFunc: func(tx *bolt.Tx) error {
				b, err := tx.CreateBucket([]byte(kind))
				if err != nil {
					return err
				}

				_, err = b.CreateBucket([]byte("some-other-key"))
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupDB()
			defer cleanup(db)

			if tt.setupFunc != nil {
				assert.NoError(t, db.Update(tt.setupFunc))
			}

			cc := &commentable{db: db, key: key, kind: kind}
			got := cc.exists()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_verify(t *testing.T) {
	t.Parallel()

	kind := "resource"
	tests := []struct {
		name      string
		setupFunc func(*bolt.Tx) error
		want      bool
	}{
		{
			name: "it returns true if resource type exists",
			setupFunc: func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte(kind))
				return err
			},
			want: true,
		},
		{
			name: "it returns false if resource type does not exist",
			setupFunc: func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte("some-other-kind"))
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupDB()
			defer cleanup(db)

			if tt.setupFunc != nil {
				assert.NoError(t, db.Update(tt.setupFunc))
			}

			got := verify(db, kind)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_setup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		exp  []bool
		want error
	}{
		{
			name: "it returns error if could not create the commentable",
			args: []string{"", ""},
			exp:  []bool{false, false},
			want: bolt.ErrBucketNameRequired,
		},
		{
			name: "it returns error if could not create the commentable",
			args: []string{"", "wont create"},
			exp:  []bool{false, false},
			want: bolt.ErrBucketNameRequired,
		},
		{
			name: "it returns true if resource type exists",
			args: []string{"commentable-1", "commentable-2"},
			exp:  []bool{true, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupDB()
			defer cleanup(db)

			got := setup(db, tt.args)
			assert.Equal(t, tt.want, got)
			
			for i, name := range tt.args {
				assert.Equal(t, tt.exp[i], verify(db, name))
			}
		})
	}
}

func Test_commentable_save(t *testing.T) {
	t.Parallel()

	kind := "commentableType"
	key := "commentableID"
	tests := []struct {
		name    string
		kind    string
		key     string
		co      *comment
		want    *comment
		wantErr error
	}{
		{
			name:    "it returns error if commentable type is not found",
			kind:    "unknown",
			co:      &comment{ID: "1234", Value: "something"},
			wantErr: fmt.Errorf(commentableTypeNotFoundFmt, "unknown"),
		},
		{
			name:    "it returns error if commentable is not found",
			kind:    kind,
			key:     "unknown",
			co:      &comment{ID: "1234", Value: "something"},
			wantErr: fmt.Errorf(commentableNotFoundFmt, "unknown", kind),
		},
		{
			name:    "it returns error if comment id is empty",
			kind:    kind,
			key:     key,
			co:      &comment{Value: "something"},
			wantErr: bolt.ErrKeyRequired,
		},
		{
			name:    "it returns error if the comment is empty",
			kind:    kind,
			key:     key,
			wantErr: fmt.Errorf("comment should not be empty"),
		},
		{
			name: "it saves the comment successfully",
			kind: kind,
			key:  key,
			co:   &comment{ID: "1234", Value: "something"},
			want: &comment{ID: "1234", Value: "something"},
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

			cm := &commentable{db: db, kind: tt.kind, key: tt.key}
			got, err := cm.save(tt.co)

			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_commentable_add(t *testing.T) {
	t.Parallel()

	kind := "commentable"
	key := "commentableID"
	tests := []struct {
		name    string
		kind    string
		key     string
		co      *comment
		wantErr error
	}{
		{
			name:    "it returns error if comemntable type is not found",
			kind:    "unknown",
			co:      &comment{Value: "some comment stuff"},
			wantErr: fmt.Errorf(commentableTypeNotFoundFmt, "unknown"),
		},
		{
			name:    "it returns error if commentable is not found",
			kind:    kind,
			key:     "unknown",
			co:      &comment{Value: "some comment stuff"},
			wantErr: fmt.Errorf(commentableNotFoundFmt, "unknown", kind),
		},
		{
			name:    "it returns error if the comment is empty",
			kind:    kind,
			key:     key,
			wantErr: fmt.Errorf("comment should not be empty"),
		},
		{
			name: "it saves the comment successfully",
			kind: kind,
			key:  key,
			co:   &comment{Value: "some comment stuff"},
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

			cm := &commentable{db: db, kind: tt.kind, key: tt.key}
			c, err := cm.add(tt.co)

			assert.Equal(t, tt.wantErr, err)
			if tt.wantErr == nil {
				assert.Equal(t, c.Value, tt.co.Value)
				assert.NotEmpty(t, c.ID)
			}
		})
	}
}

func Test_commentable_get(t *testing.T) {
	t.Parallel()

	kind := "commentable"
	key := "commentableID"
	cmt := &comment{ID: "12345", Value: "something"}
	db := setupDB()
	defer cleanup(db)

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
		name    string
		kind    string
		key     string
		cKey    string
		want    *comment
		wantErr error
	}{
		{
			name:    "it returns error if commentable type is not found",
			kind:    "unknown",
			cKey:    cmt.ID,
			wantErr: fmt.Errorf(commentableTypeNotFoundFmt, "unknown"),
		},
		{
			name:    "it returns error if commentable is not found",
			kind:    kind,
			key:     "unknown",
			cKey:    cmt.ID,
			wantErr: fmt.Errorf(commentableNotFoundFmt, kind, "unknown"),
		},
		{
			name:    "it returns error if comment with the given key is not found",
			kind:    kind,
			key:     key,
			cKey:    "unknown-key",
			wantErr: fmt.Errorf(commentNotFoundFmt, "unknown-key", kind, key),
		},
		{
			name: "it returns the comment for the given key",
			kind: kind,
			key:  key,
			cKey: cmt.ID,
			want: cmt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := &commentable{db: db, kind: tt.kind, key: tt.key}
			got, err := cm.get(tt.cKey)

			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_commentable_remove(t *testing.T) {
	t.Parallel()

	kind := "commentable"
	key := "commentableID"
	cmt := &comment{ID: "12345", Value: "something"}
	db := setupDB()
	defer cleanup(db)

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
		name    string
		kind    string
		key     string
		cKey    string
		wantErr error
	}{
		{
			name:    "it returns error if commentable type is not found",
			kind:    "unknown",
			cKey:    cmt.ID,
			wantErr: fmt.Errorf(commentableTypeNotFoundFmt, "unknown"),
		},
		{
			name:    "it returns error if commentable is not found",
			kind:    kind,
			key:     "unknown",
			cKey:    cmt.ID,
			wantErr: fmt.Errorf(commentableNotFoundFmt, "unknown", kind),
		},
		{
			name: "it removes the comment and returns no error",
			kind: kind,
			key:  key,
			cKey: cmt.ID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := &commentable{db: db, kind: tt.kind, key: tt.key}
			err := cm.remove(tt.cKey)

			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func Test_commentable_list(t *testing.T) {
	t.Parallel()

	kind := "commentable"
	key := "commentableID"

	tests := []struct {
		name      string
		kind      string
		key       string
		setupFunc func(*commentable) ([]*comment, error)
		wantErr   error
	}{
		{
			name:    "it returns error if commentable type is not found",
			kind:    "unknown",
			wantErr: fmt.Errorf(commentableTypeNotFoundFmt, "unknown"),
		},
		{
			name:    "it returns error if commentable is not found",
			kind:    kind,
			key:     "unknown",
			wantErr: fmt.Errorf(commentableNotFoundFmt, "unknown", kind),
		},
		{
			name: "it returns the comments for the given resource",
			setupFunc: func(cm *commentable) ([]*comment, error) {
				c, err := cm.add(&comment{Value: "hello world"})
				return []*comment{c}, err
			},
			kind: kind,
			key:  key,
		},
		{
			name: "it returns empty if no comment for the given resource",
			setupFunc: func(cm *commentable) ([]*comment, error) {
				return []*comment{}, nil
			},
			kind: kind,
			key:  key,
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

			cm := &commentable{db: db, kind: tt.kind, key: tt.key}

			var want []*comment
			if tt.setupFunc != nil {
				want, err = tt.setupFunc(cm)
				assert.NoError(t, err)
			}

			got, err := cm.list()

			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, want, got)
		})
	}
}
