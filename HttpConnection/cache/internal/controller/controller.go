package controller

import (
	"urlshortener.com/cache/internal/engine"
	"urlshortener.com/utils"
)

type StorageController interface {
	CreateURLValuePair(longURL engine.KeyType, shortVal engine.ValueType) error
	GetValueForURL(shortenURL engine.ValueType) (engine.KeyType, error)
}

type CacheController struct {
	cache *engine.Cache
}

func New() *CacheController {
	return &CacheController{engine.New()}
}

func (cntrl *CacheController) CreateURLValuePair(longURL engine.KeyType, shortVal engine.ValueType) utils.URLPair {
	shortURL, err := cntrl.cache.GetValueForKey(longURL)
	if err == nil {
		return utils.URLPair{LongURL: longURL, ShortURL: shortURL}
	}
	cntrl.cache.AddKeyValue(longURL, shortVal)
	return utils.URLPair{LongURL: longURL, ShortURL: shortVal}
}

func (cntrl *CacheController) GetValueForURL(shortenURL engine.ValueType) (engine.KeyType, error) {

	return cntrl.cache.GetKeyForValue(shortenURL)
}
