package main

import "time"

type RateLimit struct {
	tokensPerSec  uint
	maxTokens     uint
	currentTokens uint
	timer         time.Time
}

func NewRateLimit(tokensPerSec, maxTokens uint) *RateLimit {
	return &RateLimit{
		tokensPerSec:  tokensPerSec,
		maxTokens:     maxTokens,
		currentTokens: 0,
		timer:         time.Now(),
	}
}

func (r *RateLimit) RateLimit(tokens uint) bool {
	elapsed := time.Since(r.timer)
	if elapsed < 0 {
		r.timer = time.Now()
		return false
	}

	drainTokens := r.tokensPerSec * uint(elapsed.Milliseconds()) / 1000
	if drainTokens > 0 {
		r.timer = time.Now()
	}

	if r.currentTokens > drainTokens {
		r.currentTokens -= drainTokens
	} else {
		r.currentTokens = 0
	}

	limit := r.currentTokens+tokens > r.maxTokens
	if !limit {
		r.currentTokens += tokens
	}
	return limit
}
