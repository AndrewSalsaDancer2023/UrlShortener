package controller

import (
	"context"

	"urlshortener.com/gateway/pkg/config"
	"urlshortener.com/utils"
)

var DefaultValue = utils.ShortURL{}

type engineGateway interface {
	CreateShortURL(ctx context.Context, url string) (utils.ShortURL, error)
}

type cacheGateway interface {
	CreatURLPair(ctx context.Context, pair utils.URLPair) (utils.URLPair, error)
	GetLongURL(ctx context.Context, shortURL string) (utils.ShortURL, error)
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
		return DefaultValue, err
	}

	pair := utils.URLPair{LongURL: longURL, ShortURL: resp.URL}
	urls, err := c.cacheGateway.CreatURLPair(ctx, pair)
	if err != nil {
		return DefaultValue, err
	}

	cfg := config.GetConfig()
	url := utils.MakeURLString(cfg.ProtocolKey+cfg.DoubleSeparator, cfg.GatewayURL+cfg.OriginalURLPathShort, urls.ShortURL)

	return utils.ShortURL{URL: url}, nil
}

func (c *Controller) GetOriginalURL(ctx context.Context, shortURL string) (utils.ShortURL, error) {
	return c.cacheGateway.GetLongURL(ctx, shortURL)
}
