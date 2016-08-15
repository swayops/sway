package google

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

const (
	shortenerEndpoint = "https://www.googleapis.com/urlshortener/v1/url?key=%s"
	expandEndpoint    = "https://www.googleapis.com/urlshortener/v1/url?key=%s&shortUrl=%s"
)

var (
	ErrURL = errors.New("Invalid URL!")
)

type Load struct {
	LongURL string `json:"longUrl"`
	Id      string `json:"id"`
	Error   struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func ShortenURL(url string, cfg *config.Config) (string, error) {
	if url == "" {
		return "", ErrURL
	}

	load := &Load{LongURL: url}
	b, err := json.Marshal(load)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf(shortenerEndpoint, cfg.YouTube.ClientId)

	var respLoad Load
	err = misc.Request("POST", endpoint, string(b), &respLoad)
	if err != nil {
		return "", err
	}

	if respLoad.Error.Message != "" {
		return "", errors.New(respLoad.Error.Message)
	}

	if respLoad.Id == "" {
		return "", ErrURL
	}

	return respLoad.Id, nil
}

func ExpandURL(url string, cfg *config.Config) (string, error) {
	if url == "" {
		return "", ErrURL
	}

	endpoint := fmt.Sprintf(expandEndpoint, cfg.YouTube.ClientId, url)

	var respLoad Load
	err := misc.Request("GET", endpoint, "", &respLoad)
	if err != nil {
		return "", err
	}

	if respLoad.Error.Message != "" {
		return "", errors.New(respLoad.Error.Message)
	}

	if respLoad.LongURL == "" {
		return "", ErrURL
	}

	return respLoad.LongURL, nil
}
