package main

type rating struct {
	FiveStars  int `json:"five_stars"`
	FourStars  int `json:"four_stars"`
	ThreeStars int `json:"three_stars"`
	TwoStars   int `json:"two_stars"`
	OneStars   int `json:"one_stars"`
}

func (r *rating) add(rt rating) *rating {
	r.FiveStars += rt.FiveStars
	r.FourStars += rt.FourStars
	r.ThreeStars += rt.ThreeStars
	r.TwoStars += rt.TwoStars
	r.OneStars += rt.OneStars

	return r
}

func (r *rating) ensureNotNegative() *rating {
	if r.FiveStars < 0 {
		r.FiveStars = 0
	}
	if r.FourStars < 0 {
		r.FourStars = 0
	}
	if r.ThreeStars < 0 {
		r.ThreeStars = 0
	}
	if r.TwoStars < 0 {
		r.TwoStars = 0
	}
	if r.OneStars < 0 {
		r.OneStars = 0
	}

	return r
}
