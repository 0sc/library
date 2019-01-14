package main

import "fmt"
import "github.com/kjk/betterguid"

// ResourceName is custom type for the commentable resourses
type ResourceName int

const (
	// BOOKS represent the book resource
	BOOKS ResourceName = iota + 1
	// AUTHORS represent the author resource
	AUTHORS
	// COMMENTS represent the comment resource
	COMMENTS
)

func (rn ResourceName) String() string {
	resources := []string{"books", "authors", "comments"}
	return resources[rn]
}

type resource struct {
	name     ResourceName
	comments map[string]*comment
}

func (r *resource) list() []*comment {
	var comments []*comment
	for _, comment := range r.comments {
		comments = append(comments, comment)
	}
	return comments
}

func (r *resource) get(cID string) (*comment, error) {
	var err error
	comment, ok := r.comments[cID]
	if !ok {
		err = fmt.Errorf("comment with id %s not found for resource %s", cID, r.name)
	}

	return comment, err
}

func (r *resource) add(text string) (*comment, error) {
	if text == "" {
		return nil, fmt.Errorf("comment is empty")
	}

	if r.comments == nil {
		r.comments = make(map[string]*comment)
	}

	komment := &comment{id: betterguid.New(), text: text}
	r.comments[komment.id] = komment
	return komment, nil
}

func (r *resource) remove(cID string) error {
	if _, err := r.get(cID); err != nil {
		return err
	}

	delete(r.comments, cID)
	return nil
}
