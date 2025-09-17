package controller

import (
	"errors"

	"urlshortener.com/engine"
	"urlshortener.com/utils"
)

var ErrGenerateNumber = errors.New("unable generate random number")

const DefaultValue = ""

type Controller struct {
	generator engine.RandomGenerator
}

func New(gen engine.RandomGenerator) *Controller {
	return &Controller{generator: gen}
}

func (eng *Controller) CreateRandomValue() (string, error) {
	value, err := eng.generator.GenerateRandomValue()
	if err != nil {
		return DefaultValue, ErrGenerateNumber
	}
	return utils.ToBase62(value)
}
