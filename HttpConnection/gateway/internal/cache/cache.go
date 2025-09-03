package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"urlshortener.com/gateway/pkg/config"
	"urlshortener.com/utils"
)

var DefaultURLPair = utils.URLPair{}
var DefaultValue = utils.LongURL{}
var ErrImpossibleGenerateURL = errors.New(string("impossible to generate short url"))

const maxTries = 10

type Gateway struct {
	cacheURL     string
	shortURLPath string
}

func New(cacheURL string, shortURLPath string) *Gateway {
	return &Gateway{cacheURL: cacheURL, shortURLPath: shortURLPath}
}

func (g *Gateway) GetLongURL(ctx context.Context, longURLPath string) (*utils.LongURL, error) {

	cfg := config.GetConfig()
	longURL := utils.MakeURLString(cfg.ProtocolKey+cfg.DoubleSeparator, g.cacheURL, longURLPath)
	httpResp, err := utils.ExecuteGetRequest(ctx, longURL)

	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusOK {
		return utils.ReadURLFromResponse[utils.LongURL](httpResp)
	}

	return nil, utils.ReadErrorFromRespBody(httpResp.Body)
}

func (g *Gateway) CreatURLPair(ctx context.Context, urlpair utils.URLPair) (*utils.URLPair, error) {

	if err := utils.ValidateLongURL(urlpair.LongURL); err != nil {
		return nil, err
	}

	config := config.GetConfig()
	url := utils.MakeURLString(config.ProtocolKey+config.DoubleSeparator, g.cacheURL, g.shortURLPath)

	bodyBytes, err := json.Marshal(urlpair)
	if err != nil {
		return nil, err
	}

	curTrie := 0
	for curTrie < maxTries {
		httpResp, err := utils.ExecutePostRequest(ctx, url, bytes.NewBuffer(bodyBytes))
		if err != nil {
			return nil, err
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode == http.StatusOK {
			urls, err := utils.ReadURLFromResponse[utils.URLPair](httpResp)
			if err != nil {
				return nil, err
			}

			if urls.LongURL != urlpair.LongURL {
				curTrie++
				continue
			}

			if err = utils.ValidateShortURL(urls.ShortURL); err != nil {
				return nil, err
			}
			return urls, nil
		}

		return nil, utils.ReadErrorFromRespBody(httpResp.Body)
	}
	return nil, ErrImpossibleGenerateURL
}
