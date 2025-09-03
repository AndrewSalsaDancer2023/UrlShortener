package utils

import (
	"bytes"
	"context"
	"encoding/json"
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

var ErrGetRequest = "error sending GET request: %w"
var ErrPostRequest = "error sending POST request: %w"

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

func ExecuteRequest(req *http.Request, errDescr string) (*http.Response, error) {
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return nil, fmt.Errorf(errDescr, err)
	}

	return resp, nil
}

func ExecuteGetRequest(context context.Context, url string) (*http.Response, error) {
	reqGet, err := http.NewRequestWithContext(context, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid new GET request: %w", err)
	}

	return ExecuteRequest(reqGet, ErrGetRequest)
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

	return ExecuteRequest(reqPost, ErrPostRequest)
}

func MakeURLString(protocol string, host string, path string) string {
	return protocol + host + path
}

func WriteErrorResponse(statusCode int, errorResp string, w http.ResponseWriter) {
	w.WriteHeader(statusCode)
	w.Write([]byte(errorResp))
}

type URLType interface {
	ShortURL | LongURL | URLPair
}

func ReadURL[urlType URLType](statusCode int, w http.ResponseWriter, req *http.Request) (urlType, error) {
	var url urlType
	if err := json.NewDecoder(req.Body).Decode(&url); err != nil {
		WriteErrorResponse(statusCode, err.Error(), w)
		return url, err
	}

	return url, nil
}

func WriteURL[urlType URLType](urls *urlType, statusCode int, w http.ResponseWriter) {
	if err := json.NewEncoder(w).Encode(urls); err != nil {
		WriteErrorResponse(statusCode, err.Error(), w)
	}
}

func ReadErrorFromRespBody(body io.ReadCloser) error {
	bodyBytes, err := io.ReadAll(body)

	if err != nil {
		return err
	}
	return errors.New(string(bodyBytes))
}

func ReadURLFromResponse[urlType URLType](httpResp *http.Response) (*urlType, error) {
	var urls urlType
	if err := json.NewDecoder(httpResp.Body).Decode(&urls); err != nil {
		return nil, err
	}
	return &urls, nil
}
