package utils

import (
	"errors"
	"strings"
)

const (
	alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	base     = int64(len(alphabet)) // 62
)

// ToBase62 кодирует Snowflake ID в строку Base62.
// Snowflake ID всегда положителен, поэтому отрицательный вход — ошибка.
//
// Максимальный Snowflake ID (63 значимых бита) = 9_223_372_036_854_775_807,
// что кодируется в 10-11 символов Base62.
func ToBase62(id int64) (string, error) {
	if id < 0 {
		return "", errors.New("id must be non-negative")
	}
	if id == 0 {
		return "0", nil
	}

	var buf [11]byte // массив на стеке, не вызывает аллокацию
	i := len(buf)

	for id > 0 {
		i--
		buf[i] = alphabet[id%base]
		id /= base
	}

	return string(buf[i:]), nil
}

// FromBase62 декодирует строку Base62 обратно в int64.
// Возвращает ошибку, если строка содержит символы вне алфавита.
func FromBase62(s string) (int64, error) {
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
