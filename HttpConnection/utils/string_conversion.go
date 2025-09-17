package utils

import (
	"errors"
	"strings"
)

const DefaultValue = ""
const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const base = 62

var ErrConvertNumber = errors.New("unable convert number to string")
var ErrConvertString = errors.New("unable convert string to number")

func convertBase62IntToString(reminder uint64) (string, error) {
	if reminder >= uint64(len(alphabet)) {
		return "", ErrConvertNumber
	}
	return string(alphabet[reminder]), nil
}

func ToBase62(number uint64) (string, error) {
	if number == 0 {
		return string(alphabet[number]), nil
	}

	var res string = ""

	for number != 0 {
		reminder := number % base
		number = number / base
		val, err := convertBase62IntToString(reminder)
		if err != nil {
			return DefaultValue, err
		}
		res = val + res
	}

	return res, nil
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
