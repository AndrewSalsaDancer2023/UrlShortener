package cache

import (
	"context"
	"net/http"

	"urlshortener.com/gateway/pkg/config"
	"urlshortener.com/utils"
)

type Gateway struct {
	engineURL    string
	shortURLPath string
}

func New(engineURL string, shortURLPath string) *Gateway {
	return &Gateway{engineURL: engineURL, shortURLPath: shortURLPath}
}

func (g *Gateway) CreateShortURL(ctx context.Context, url string) (*utils.ShortURL, error) {

	if err := utils.ValidateLongURL(url); err != nil {
		return nil, err
	}

	config := config.GetConfig()
	url = utils.MakeURLString(config.ProtocolKey+config.DoubleSeparator, g.engineURL, g.shortURLPath)
	resp, err := utils.ExecutePostRequest(ctx, url, nil)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return utils.ReadURLFromResponse[utils.ShortURL](resp)
	}

	return nil, utils.ReadErrorFromRespBody(resp.Body)
}
