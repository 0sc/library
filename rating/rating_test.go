package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_rating_add(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		arg  rating
		want *rating
	}{
		{
			name: "it updates it's values with the given rating - 1",
			arg:  rating{FiveStars: 2, FourStars: 1, ThreeStars: 1000, TwoStars: 976, OneStars: 12},
			want: &rating{FiveStars: 3, FourStars: 3, ThreeStars: 1003, TwoStars: 980, OneStars: 17},
		},
		{
			name: "it updates it's values with the given rating - 2",
			arg:  rating{FiveStars: -2, FourStars: 1, ThreeStars: 10, TwoStars: -6, OneStars: 0},
			want: &rating{FiveStars: -1, FourStars: 3, ThreeStars: 13, TwoStars: -2, OneStars: 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &rating{
				FiveStars:  1,
				FourStars:  2,
				ThreeStars: 3,
				TwoStars:   4,
				OneStars:   5,
			}

			assert.Equal(t, tt.want, rt.add(tt.arg))
		})
	}
}

func Test_rating_ensureNotNegative(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		arg  rating
		want *rating
	}{
		{
			name: "it resets negative values to zero ",
			arg:  rating{FiveStars: -2, FourStars: -1, ThreeStars: -1000, TwoStars: -976, OneStars: -12},
			want: &rating{},
		},
		{
			name: "it makes no changes to values greater or equal to zero",
			arg:  rating{FiveStars: -2, FourStars: 1, ThreeStars: 10, TwoStars: -6, OneStars: 0},
			want: &rating{FiveStars: 0, FourStars: 1, ThreeStars: 10, TwoStars: 0, OneStars: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			assert.Equal(t, tt.want, tt.arg.ensureNotNegative())
		})
	}
}
