package imagga

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/swayops/sway/internal/common"
)

const (
	apiKey    = "acc_8352c3bab4a806a"
	apiSecret = "41f8b7289dcbffd3b7ac16f45d2c4418"
)

var (
	ErrResponse = errors.New("Empty response!")
	ErrId       = errors.New("Influencer ID in meta data does not match!!")
	client      = &http.Client{}
)

type Load struct {
	Results []struct {
		TaggingID interface{} `json:"tagging_id"`
		Image     string      `json:"image"`
		Tags      []struct {
			Confidence float64 `json:"confidence"`
			Tag        string  `json:"tag"`
		} `json:"tags"`
	} `json:"results"`
}

func GetKeywords(images []string, sandbox bool) (keywords []string, err error) {
	if sandbox {
		keywords = []string{"sandbox"}
		return
	}

	if len(images) == 0 {
		return
	}

	if len(images) > 5 {
		// Max of 5 images can be sent in one request
		images = images[:5]
	}

	params := &url.Values{}
	for _, i := range images {
		params.Add("url", i)
	}

	url := "https://api.imagga.com/v1/tagging?" + params.Encode()

	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(apiKey, apiSecret)

	resp, err := client.Do(req)

	if err != nil {
		fmt.Println("Error when sending request to the server")
		return
	}

	var load Load
	defer resp.Body.Close()
	if err = json.NewDecoder(resp.Body).Decode(&load); err != nil {
		return
	}

	if len(load.Results) > 0 {
		for _, res := range load.Results {
			for _, tag := range res.Tags {
				if tag.Confidence > 15 {
					keywords = append(keywords, tag.Tag)
				}
			}
		}
	}

	keywords = clean(keywords)
	return
}

func clean(keywords []string) []string {
	out := []string{}
	for _, kw := range keywords {
		if cleanedKw := strings.ToLower(kw); !common.IsInList(out, cleanedKw) {
			out = append(out, cleanedKw)

		}
	}
	return out
}
