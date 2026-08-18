package pool

import (
	"context"
	"time"
)

type DBTime interface {
	Now() time.Time
}

type DBTimeReal struct{}

func (DBTimeReal) Now() time.Time {
	return time.Now().UTC() // Рекомендуется всегда работать в UTC
}

type UnixTimeReal struct{}

func (UnixTimeReal) Now() time.Time {
	return time.Now()
}

