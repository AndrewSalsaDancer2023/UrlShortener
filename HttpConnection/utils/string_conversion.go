package utils

import (
	"errors"
	"math"
	"strings"
)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const base = 62

var ErrConvertNumber = errors.New("unable convert number to string")
var ErrConvertString = errors.New("unable convert string to number")

func ToBase62(number uint64) (string, error) {
	reminder := number % base
	result := string(alphabet[reminder])
	div := number / base
	num := uint64(math.Floor(float64(div)))

	for num != 0 {
		reminder = num % base
		temp := num / base
		num = uint64(math.Floor(float64(temp)))

		if int(reminder) < 0 || int(reminder) >= len(alphabet) {
			return "", ErrConvertNumber
		}
		result = string(alphabet[int(reminder)]) + result
	}

	return string(result), nil
}

func ToBase10(str string) (uint64, error) {
	var res uint64 = 0
	for _, r := range str {
		index := strings.Index(alphabet, string(r))
		if index < 0 {
			return 0, ErrConvertString
		}
		res = (base * res) + uint64(index)
	}

	return res, nil
}
