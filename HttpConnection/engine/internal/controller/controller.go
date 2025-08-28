package controller

import (
	"errors"

	cr "urlshortener.com/engine/internal/engine"
	"urlshortener.com/utils"
)

var ErrGenerateNumber = errors.New("unable generate big random number")

const DefaultValue = ""

type Controller struct {
}

func New() *Controller {
	return &Controller{}
}

func (eng *Controller) CreateRandomValue() (string, error) {
	value, err := cr.GenerateRandomValue()
	if err != nil {
		return DefaultValue, ErrGenerateNumber
	}
	return utils.ToBase62(value)
}
