package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"unicode/utf8"
)

const (
	longURLMaxLength = 100
	shortURLLength   = 7
)

var ErrURLTooLong = errors.New("URL exceed max length")
var ErrShortURLLength = errors.New("invalid URL length")
var ErrShortURLFormat = errors.New("invalid URL format")

type URLPair struct {
	LongURL  string `json:"long_url"`
	ShortURL string `json:"short_url"`
}

type ShortURL struct {
	URL string `json:"short_url"`
}

type LongURL struct {
	URL string `json:"long_url"`
}

func runeLength(source string) int {
	return utf8.RuneCountInString(source)
}

func ValidateShortURL(shortURL string) error {
	if runeLength(shortURL) != shortURLLength {
		return ErrShortURLLength
	}

	for _, r := range shortURL {
		if ('0' <= r && r <= '9') || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') {
			continue
		} else {
			return ErrShortURLFormat
		}
	}

	return nil
}

func ValidateLongURL(rawURL string) error {
	if runeLength(rawURL) > longURLMaxLength {
		return ErrURLTooLong
	}
	_, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	return nil
}

func ExecuteGetRequest(context context.Context, url string) (*http.Response, error) {
	reqGet, err := http.NewRequestWithContext(context, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid new GET request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(reqGet)

	if err != nil {
		return nil, fmt.Errorf("error sending GET request: %w", err)
	}

	return resp, nil
}

func ExecutePostRequest(context context.Context, url string, reqbody *bytes.Buffer) (*http.Response, error) {

	if reqbody == nil {
		reqbody = &bytes.Buffer{}
	}
	reqPost, err := http.NewRequestWithContext(context, "POST", url, reqbody)
	if err != nil {
		return nil, fmt.Errorf("invalid new POST request: %w", err)
	}
	reqPost.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(reqPost)

	if err != nil {
		return nil, fmt.Errorf("error sending POST request: %w", err)
	}

	return resp, nil
}

func CopyRequestBody(body io.ReadCloser) (*bytes.Buffer, error) {
	LongURL := new(bytes.Buffer)
	_, err := io.Copy(LongURL, body)
	if err != nil {
		return nil, err
	}

	return LongURL, nil
}

func MakeURLString(protocol string, host string, path string) string {
	return protocol + host + path
}
