package main

type config struct {
	Port int    `default:"50050"`
	DSN  string `default:"db/comments.db"`
}
