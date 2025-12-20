package fetcher

import "time"

type Config struct {
	MaxConcurrency int
	CommitSize     int
	ScoreTimeout   time.Duration
}
