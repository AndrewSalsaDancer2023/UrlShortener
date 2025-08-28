package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"urlshortener.com/gateway/pkg/config"
	"urlshortener.com/utils"
)

var DefaultURLPair = utils.URLPair{}
var DefaultValue = utils.ShortURL{}
var ImpossibleGenerateURLError = errors.New(string("impossible to generate short url"))

const maxTries = 10

type CacheGatewayInterface interface {
	GetLongURL(ctx context.Context, longURLPath string) (utils.ShortURL, error)
	CreatURLPair(ctx context.Context, urlpair utils.URLPair) (utils.URLPair, error)
}

type Gateway struct {
	CacheGatewayInterface
	cacheURL     string
	shortURLPath string
}

func New(cacheURL string, shortURLPath string) *Gateway {
	return &Gateway{cacheURL: cacheURL, shortURLPath: shortURLPath}
}

func (g *Gateway) GetLongURL(ctx context.Context, longURLPath string) (utils.ShortURL, error) {

	cfg := config.GetConfig()
	getLongURL := utils.MakeURLString(cfg.ProtocolKey+cfg.DoubleSeparator, g.cacheURL, longURLPath)
	httpResp, err := utils.ExecuteGetRequest(ctx, getLongURL)

	if err != nil {
		return DefaultValue, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusOK {
		var data utils.ShortURL
		if err := json.NewDecoder(httpResp.Body).Decode(&data); err != nil {
			return DefaultValue, err
		}
		return data, nil
	}

	errorDescr, err := io.ReadAll(httpResp.Body)

	if err != nil {
		return DefaultValue, err
	}

	return DefaultValue, errors.New(string(errorDescr))

}

func (g *Gateway) CreatURLPair(ctx context.Context, urlpair utils.URLPair) (utils.URLPair, error) {

	if err := utils.ValidateLongURL(urlpair.LongURL); err != nil {
		return DefaultURLPair, err
	}

	config := config.GetConfig()
	url := utils.MakeURLString(config.ProtocolKey+config.DoubleSeparator, g.cacheURL, g.shortURLPath)

	bodyBytes, err := json.Marshal(urlpair)
	if err != nil {
		return DefaultURLPair, err
	}

	curTrie := 0
	for curTrie < maxTries {
		httpResp, err := utils.ExecutePostRequest(ctx, url, bytes.NewBuffer(bodyBytes))
		if err != nil {
			return DefaultURLPair, err
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode == http.StatusOK {
			var urls utils.URLPair
			if err := json.NewDecoder(httpResp.Body).Decode(&urls); err != nil {
				return DefaultURLPair, err
			}

			if urls.LongURL != urlpair.LongURL {
				curTrie++
				continue
			}

			err = utils.ValidateShortURL(urls.ShortURL)
			if err != nil {
				return DefaultURLPair, err
			}
			return urls, nil
		}

		errorDescr, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return DefaultURLPair, err
		}

		return DefaultURLPair, errors.New(string(errorDescr))
	}
	return DefaultURLPair, ImpossibleGenerateURLError
}
