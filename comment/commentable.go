package main

import (
	"encoding/json"
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/kjk/betterguid"
)

var (
	commentableNotFoundFmt     = "%s not found with key %s"
	commentableTypeNotFoundFmt = "commentable type, %s, not found"
	commentNotFoundFmt         = "comment with key %s not found for %s with id %s"
	commentsKey                = []byte("comments")
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

type commentable struct {
	kind string // author, books
	key  string // resource id
	db   *bolt.DB
}

func (cm *commentable) ensure() error {
	return cm.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(cm.kind))
		if bucket == nil {
			return fmt.Errorf("resource '%s' does not exist", cm.kind)
		}

		_, err := bucket.CreateBucketIfNotExists([]byte(cm.key))
		return err
	})
}

func (cm *commentable) exists() (found bool) {
	cm.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(cm.kind))
		if bucket != nil && bucket.Bucket([]byte(cm.key)) != nil {
			found = true
		}

		return nil
	})

	return
}

func (cm *commentable) add(c *comment) (*comment, error) {
	if c == nil {
		return nil, fmt.Errorf("comment should not be empty")
	}

	c.ID = betterguid.New()
	return cm.save(c)
}

func (cm *commentable) save(c *comment) (*comment, error) {
	if c == nil {
		return nil, fmt.Errorf("comment should not be empty")
	}

	err := cm.db.Update(func(tx *bolt.Tx) error {
		cmBucket := tx.Bucket([]byte(cm.kind)) // bucket for posts
		if cmBucket == nil {
			return fmt.Errorf(commentableTypeNotFoundFmt, cm.kind)
		}

		rBucket := cmBucket.Bucket([]byte(cm.key)) // subbucket for post with key
		if rBucket == nil {
			return fmt.Errorf(commentableNotFoundFmt, cm.key, cm.kind)
		}

		comments, err := rBucket.CreateBucketIfNotExists(commentsKey) // prep the comments subbucket
		if err != nil {
			return fmt.Errorf("error setting up comments for %s with key %s %v", cm.kind, cm.key, err)
		}

		data, err := json.Marshal(c)
		if err != nil {
			return fmt.Errorf("error preparing comment %v, %v", c, err)
		}

		return comments.Put([]byte(c.ID), data)
	})

	// clear out the comment if error occured
	if err != nil {
		c = nil
	}

	return c, err
}

func (cm *commentable) list() ([]*comment, error) {
	var comments []*comment
	err := cm.db.View(func(tx *bolt.Tx) error {
		cmBucket := tx.Bucket([]byte(cm.kind)) // bucket for posts
		if cmBucket == nil {
			return fmt.Errorf(commentableTypeNotFoundFmt, cm.kind)
		}

		rBucket := cmBucket.Bucket([]byte(cm.key)) // subbucket for post with key
		if rBucket == nil {
			return fmt.Errorf(commentableNotFoundFmt, cm.key, cm.kind)
		}

		comments = []*comment{}
		komments := rBucket.Bucket(commentsKey)
		if komments == nil {
			return nil
		}

		return komments.ForEach(func(_, data []byte) error {
			var c comment
			err := json.Unmarshal(data, &c)
			if err != nil {
				return err
			}

			comments = append(comments, &c)
			return nil
		})
	})

	return comments, err
}

func (cm *commentable) get(cKey string) (c *comment, err error) {
	err = cm.db.View(func(tx *bolt.Tx) error {
		cmBucket := tx.Bucket([]byte(cm.kind)) // bucket for posts
		if cmBucket == nil {
			return fmt.Errorf(commentableTypeNotFoundFmt, cm.kind)
		}

		rBucket := cmBucket.Bucket([]byte(cm.key)) // subbucket for post with key
		if rBucket == nil {
			return fmt.Errorf(commentableNotFoundFmt, cm.kind, cm.key)
		}

		comments := rBucket.Bucket(commentsKey) // prep the comments subbucket
		if comments == nil {
			return fmt.Errorf(commentNotFoundFmt, cKey, cm.kind, cm.key)
		}

		cmm := comments.Get([]byte(cKey))
		if cmm == nil {
			return fmt.Errorf(commentNotFoundFmt, cKey, cm.kind, cm.key)
		}

		c = &comment{}
		return json.Unmarshal(cmm, c)
	})

	return c, err
}

func (cm *commentable) remove(cKey string) error {
	return cm.db.Update(func(tx *bolt.Tx) error {
		cmBucket := tx.Bucket([]byte(cm.kind)) // bucket for posts
		if cmBucket == nil {
			return fmt.Errorf(commentableTypeNotFoundFmt, cm.kind)
		}

		rBucket := cmBucket.Bucket([]byte(cm.key)) // subbucket for post with key
		if rBucket == nil {
			return fmt.Errorf(commentableNotFoundFmt, cm.key, cm.kind)
		}

		comments := rBucket.Bucket(commentsKey) // prep the comments subbucket
		if comments == nil {
			return fmt.Errorf("comment with key %s not found for %s resource with id %s", cKey, cm.kind, cm.key)
		}

		return comments.Delete([]byte(cKey))
	})

}
