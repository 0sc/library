package main

import (
	"reflect"
	"testing"
)

func Test_resource_list(t *testing.T) {
	t.Parallel()

	komment := &comment{id: "comment-1", text: "some text"}
	res := &resource{
		comments: map[string]*comment{komment.id: komment},
	}
	tests := []struct {
		name string
		want []*comment
	}{
		{
			name: "it returns all the comment for the resource",
			want: []*comment{komment},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := res.list(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("resource.list() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_resource_get(t *testing.T) {
	t.Parallel()

	komment := &comment{id: "comment-1"}

	tests := []struct {
		name     string
		arg      string
		comments map[string]*comment
		want     *comment
		wantErr  bool
	}{
		{
			name:    "it returns error if no comment exists",
			arg:     komment.id,
			want:    nil,
			wantErr: true,
		},
		{
			name:     "it returns error if comment with id not found",
			arg:      komment.id,
			comments: map[string]*comment{"comment-2": komment},
			want:     nil,
			wantErr:  true,
		},
		{
			name:     "it returns the comment for the given comment id",
			arg:      komment.id,
			comments: map[string]*comment{komment.id: komment},
			want:     komment,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := resource{comments: tt.comments}
			got, err := r.get(tt.arg)

			if (err != nil) != tt.wantErr {
				t.Errorf("resource.get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("resource.get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_resource_add(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		arg      string
		comments map[string]*comment
		wantErr  bool
		wantLen  int
	}{
		{
			name:    "it returns error is text is empty",
			wantErr: true,
		},
		{
			name: "it adds the comment to existing comments",
			arg:  "new comment",
			comments: map[string]*comment{
				"comemnt-1": &comment{id: "comment-1"},
			},
			wantLen: 2,
		},
		{
			name:    "it creates a new comments list with the comment",
			arg:     "new comment",
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := resource{comments: tt.comments}
			got, err := res.add(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("resource.add() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			gotLen := len(res.comments)
			if gotLen != tt.wantLen {
				t.Errorf("len(resource.comments) = %d, want %d", gotLen, tt.wantLen)
				return
			}

			if tt.wantLen != 0 && tt.arg != got.text {
				t.Errorf("want saved comment.text = %s, got %s", tt.arg, got.text)
			}
		})
	}
}

func Test_resource_remove(t *testing.T) {
	t.Parallel()

	comments := map[string]*comment{
		"comment-1": &comment{},
		"comment-2": &comment{},
	}

	tests := []struct {
		name    string
		arg     string
		wantErr bool
		wantLen int
	}{
		{
			name:    "it returns error if comment with id not found",
			arg:     "comment-3",
			wantErr: true,
			wantLen: 2,
		},
		{
			name:    "it removes the comment from the list",
			arg:     "comment-2",
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			res := &resource{comments: comments}
			err := res.remove(tt.arg)

			if (err != nil) != tt.wantErr {
				t.Errorf("resource.remove() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if gotLen := len(res.comments); gotLen != tt.wantLen {
				t.Errorf("want len(res.remove) = %d, got %d", tt.wantLen, gotLen)
			}
		})
	}
}
