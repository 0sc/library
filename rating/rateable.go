package main

import (
	"encoding/json"
	"fmt"

	"github.com/boltdb/bolt"
)

var (
	rateableTypeNotFoundFmt = "rateable type, %s, not found"
	rateableNotFoundFmt     = "%s not found with key %s"
	ratingsKey              = []byte("ratings")
)

func setup(db *bolt.DB, cmts []string) error {
	return db.Update(func(tx *bolt.Tx) error {
		for _, b := range cmts {
			_, err := tx.CreateBucketIfNotExists([]byte(b))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func verify(db *bolt.DB, kind string) (found bool) {
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(kind))
		found = b != nil
		return nil
	})

	return
}

type rateable struct {
	kind string // author, books
	key  string // resource id
	db   *bolt.DB
}

func (r *rateable) save(rt rating) (*rating, error) {
	var newRating *rating
	err := r.db.Update(func(tx *bolt.Tx) error {
		rtBucket := tx.Bucket([]byte(r.kind))
		if rtBucket == nil {
			return fmt.Errorf(rateableTypeNotFoundFmt, r.kind)
		}

		rBucket, err := rtBucket.CreateBucketIfNotExists([]byte(r.key))
		if err != nil {
			return err
		}

		var currentRating rating
		data := rBucket.Get(ratingsKey)
		if data != nil {
			if err = json.Unmarshal(data, &currentRating); err != nil {
				return err
			}
		}

		newRating = currentRating.add(rt).ensureNotNegative()
		data, err = json.Marshal(newRating)
		if err != nil {
			return err
		}

		return rBucket.Put(ratingsKey, data)
	})

	return newRating, err
}

func (r *rateable) get() (*rating, error) {
	var rt *rating

	err := r.db.View(func(tx *bolt.Tx) error {
		rtBucket := tx.Bucket([]byte(r.kind)) // bucket for resource type
		if rtBucket == nil {
			return fmt.Errorf(rateableTypeNotFoundFmt, r.kind)
		}

		rBucket := rtBucket.Bucket([]byte(r.key))
		if rBucket == nil {
			return fmt.Errorf(rateableNotFoundFmt, r.kind, r.key)
		}

		rt = &rating{}
		data := rBucket.Get(ratingsKey)
		if data == nil {
			return nil
		}

		return json.Unmarshal(data, rt)
	})

	return rt, err
}
