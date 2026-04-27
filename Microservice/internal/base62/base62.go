package base62

import (
	"errors"
	"strings"
)

const (
	alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	base     = int64(len(alphabet)) // 62
)

// Encoder — интерфейс кодировщика. Позволяет подменять реализацию в тестах.
type Encoder interface {
	Encode(id int64) (string, error)
	Decode(s string) (int64, error)
}

// Base62Encoder — реализация кодирования в Base62.
type Base62Encoder struct{}

func New() *Base62Encoder {
	return &Base62Encoder{}
}

func (e *Base62Encoder) Encode(id int64) (string, error) {
	if id < 0 {
		return "", errors.New("id must be non-negative")
	}
	if id == 0 {
		return "0", nil
	}

	var buf [11]byte // максимум 11 символов для math.MaxInt64
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = alphabet[id%base]
		id /= base
	}

	return string(buf[i:]), nil
}

func (e *Base62Encoder) Decode(s string) (int64, error) {
	if s == "" {
		return 0, errors.New("input string is empty")
	}

	var result int64
	for _, ch := range s {
		idx := strings.IndexRune(alphabet, ch)
		if idx == -1 {
			return 0, errors.New("invalid character: " + string(ch))
		}
		result = result*base + int64(idx)
		if result < 0 {
			return 0, errors.New("overflow: value exceeds int64")
		}
	}

	return result, nil
}
