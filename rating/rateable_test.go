package main

import (
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

func Test_setup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		exp  []bool
		want error
	}{
		{
			name: "it returns error if could not create the rateable",
			args: []string{"", ""},
			exp:  []bool{false, false},
			want: bolt.ErrBucketNameRequired,
		},
		{
			name: "it returns error if could not create the rateable",
			args: []string{"", "wont create"},
			exp:  []bool{false, false},
			want: bolt.ErrBucketNameRequired,
		},
		{
			name: "it returns true if resource type exists",
			args: []string{"rateable-1", "rateable-2"},
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

func Test_verify(t *testing.T) {
	t.Parallel()

	kind := "rateable"
	tests := []struct {
		name      string
		setupFunc func(*bolt.Tx) error
		want      bool
	}{
		{
			name: "it returns true if rateable type exists",
			setupFunc: func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte(kind))
				return err
			},
			want: true,
		},
		{
			name: "it returns false if rateable type does not exist",
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

func Test_rateable_save(t *testing.T) {
	t.Parallel()

	kind := "rateable"
	key := "rateableKey"
	rt := rating{
		FiveStars:  1,
		FourStars:  2,
		ThreeStars: 3,
		TwoStars:   4,
		OneStars:   5,
	}

	tests := []struct {
		name      string
		key       string
		setupFunc func(*bolt.Tx) error
		want      *rating
		wantErr   error
	}{
		{
			name:    "it returns error if rateable type does not exist",
			key:     key,
			wantErr: fmt.Errorf(rateableTypeNotFoundFmt, kind),
		},
		{
			name: "it creates and saves rating if rateable does not already exist",
			setupFunc: func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte(kind))
				return err
			},
			key:  key,
			want: &rt,
		},
		{
			name: "it returns error if it cannot create rateably that does not exist",
			setupFunc: func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte(kind))
				return err
			},
			wantErr: bolt.ErrBucketNameRequired,
		},
		{
			name: "it updates the rating for already existing rateable",
			setupFunc: func(tx *bolt.Tx) error {
				b, err := tx.CreateBucket([]byte(kind))
				if err != nil {
					return err
				}

				bk, err := b.CreateBucket([]byte(key))
				if err != nil {
					return err
				}

				bk.Put(ratingsKey, []byte(`{"five_stars":5,"four_stars":4, "three_stars":3,"two_stars":2,"one_stars":1}`))
				return err
			},
			key: key,
			want: &rating{
				FiveStars:  6,
				FourStars:  6,
				ThreeStars: 6,
				TwoStars:   6,
				OneStars:   6,
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

			r := &rateable{db: db, kind: kind, key: tt.key}
			got, err := r.save(rt)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func Test_rateable_get(t *testing.T) {
	t.Parallel()

	kind := "rateable"
	key := "rateableKey"
	rt := &rating{
		FiveStars:  1,
		FourStars:  2,
		ThreeStars: 3,
		TwoStars:   4,
		OneStars:   5,
	}

	tests := []struct {
		name      string
		setupFunc func(*bolt.Tx) error
		want      *rating
		wantErr   error
	}{
		{
			name:    "it returns error if rateable type does not exist",
			wantErr: fmt.Errorf(rateableTypeNotFoundFmt, kind),
		},
		{
			name: "it returns error if rateable is not found",
			setupFunc: func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte(kind))
				return err
			},
			wantErr: fmt.Errorf(rateableNotFoundFmt, kind, key),
		},
		{
			name: "it returns rating if empty",
			setupFunc: func(tx *bolt.Tx) error {
				b, err := tx.CreateBucket([]byte(kind))
				if err != nil {
					return err
				}

				_, err = b.CreateBucket([]byte(key))
				return err
			},
			want: &rating{},
		},
		{
			name: "it returns the existing rating",
			setupFunc: func(tx *bolt.Tx) error {
				b, err := tx.CreateBucket([]byte(kind))
				if err != nil {
					return err
				}

				rb, err := b.CreateBucket([]byte(key))
				return rb.Put(ratingsKey, []byte(`{"five_stars":1,"four_stars":2, "three_stars":3,"two_stars":4,"one_stars":5}`))
			},
			want: rt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupDB()
			defer cleanup(db)

			if tt.setupFunc != nil {
				assert.NoError(t, db.Update(tt.setupFunc))
			}

			r := &rateable{db: db, kind: kind, key: key}
			got, err := r.get()
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
