package crypto_engine

import (
	"crypto/rand"
	"math/big"
)

const upperValue int64 = 3500000000000

func generateRandom(upperLimit int64) (uint64, error) {
	maxValue := big.NewInt(upperLimit)
	randomNumber, err := rand.Int(rand.Reader, maxValue)
	if err != nil {
		return 0, err
	}
	return randomNumber.Uint64(), nil
}

func GenerateRandomValue() (uint64, error) {
	return generateRandom(upperValue)
}
