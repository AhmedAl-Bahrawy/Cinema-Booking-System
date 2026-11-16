package db

import "time"

type Show struct {
	ID         int64
	Name       string
	Duration   time.Duration // per-minutes
	AuthorName string
}
