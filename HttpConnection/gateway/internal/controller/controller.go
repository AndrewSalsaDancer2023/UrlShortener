package controller

import (
	"context"
	"errors"

	"urlshortener.com/gateway/pkg/config"
	"urlshortener.com/utils"
)

var defaultShortURL = utils.ShortURL{}
var defaultLongURL = utils.LongURL{}

var errImpossibleCreateShortURL = errors.New(string("impossible to create short url"))
var errImpossibleCreateLongURL = errors.New(string("impossible to create long url"))
var errImpossibleCreateURLPair = errors.New(string("impossible to create url pair"))

type engineGateway interface {
	CreateShortURL(ctx context.Context, url string) (*utils.ShortURL, error)
}

type cacheGateway interface {
	CreatURLPair(ctx context.Context, pair utils.URLPair) (*utils.URLPair, error)
	GetLongURL(ctx context.Context, shortURL string) (*utils.LongURL, error)
}

type Controller struct {
	cacheGateway  cacheGateway
	engineGateway engineGateway
}

func New(cacheGateway cacheGateway, engineGateway engineGateway) *Controller {
	return &Controller{cacheGateway, engineGateway}
}

func (c *Controller) CreateShortURL(ctx context.Context, longURL string) (utils.ShortURL, error) {

	resp, err := c.engineGateway.CreateShortURL(ctx, longURL)
	if err != nil {
		return defaultShortURL, err
	}

	if resp == nil {
		return defaultShortURL, errImpossibleCreateShortURL
	}

	pair := utils.URLPair{LongURL: longURL, ShortURL: resp.URL}
	urls, err := c.cacheGateway.CreatURLPair(ctx, pair)
	if err != nil {
		return defaultShortURL, err
	}

	if urls == nil {
		return defaultShortURL, errImpossibleCreateURLPair
	}
	cfg := config.GetConfig()
	url := utils.MakeURLString(cfg.ProtocolKey+cfg.DoubleSeparator, cfg.GatewayURL+cfg.OriginalURLPathShort, urls.ShortURL)

	return utils.ShortURL{URL: url}, nil
}

func (c *Controller) GetLongURL(ctx context.Context, shortURL string) (utils.LongURL, error) {
	url, err := c.cacheGateway.GetLongURL(ctx, shortURL)
	if err != nil {
		return defaultLongURL, err
	}
	if url == nil {
		return defaultLongURL, errImpossibleCreateLongURL
	}
	return *url, nil
}
