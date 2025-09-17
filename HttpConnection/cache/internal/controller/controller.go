package controller

import (
	"urlshortener.com/cache"
	"urlshortener.com/utils"
)

type CacheController struct {
	cache cache.Cacher
}

func New(ch cache.Cacher) *CacheController {
	return &CacheController{cache: ch}
}

func (cntrl *CacheController) CreateURLValuePair(longURL cache.KeyType, shortVal cache.ValueType) utils.URLPair {
	shortVal = cntrl.cache.AddKeyValue(longURL, shortVal)
	return utils.URLPair{LongURL: longURL, ShortURL: shortVal}
}

func (cntrl *CacheController) GetValueForURL(shortenURL cache.ValueType) (cache.KeyType, error) {

	return cntrl.cache.GetKeyForValue(shortenURL)
}
