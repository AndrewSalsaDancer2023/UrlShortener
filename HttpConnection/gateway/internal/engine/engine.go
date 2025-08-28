package cache

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"urlshortener.com/gateway/pkg/config"
	"urlshortener.com/utils"
)

var DefaultResponce = utils.ShortURL{}

type EngineGatewayInterface interface {
	CreateShortURL(ctx context.Context, url string) (utils.ShortURL, error)
}

type Gateway struct {
	EngineGatewayInterface
	engineURL    string
	shortURLPath string
}

func New(engineURL string, shortURLPath string) *Gateway {
	return &Gateway{engineURL: engineURL, shortURLPath: shortURLPath}
}

func (g *Gateway) CreateShortURL(ctx context.Context, url string) (utils.ShortURL, error) {

	err := utils.ValidateLongURL(url)
	if err != nil {
		return DefaultResponce, err
	}

	config := config.GetConfig()
	url = utils.MakeURLString(config.ProtocolKey+config.DoubleSeparator, g.engineURL, g.shortURLPath)
	resp, err := utils.ExecutePostRequest(ctx, url, nil)

	if err != nil {
		return DefaultResponce, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var url utils.ShortURL
		if err := json.NewDecoder(resp.Body).Decode(&url); err != nil {
			return DefaultResponce, err
		}
		return url, nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return DefaultResponce, err
	}
	return DefaultResponce, errors.New(string(bodyBytes))
}
