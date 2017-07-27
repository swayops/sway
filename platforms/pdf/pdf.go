package pdf

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/swayops/sway/config"
)

const (
	testEndpoint = "http://api.pdflayer.com/api/convert?access_key=2b82226c6c03b23482e984c258866fc2&force=1&page_width=684&page_height=864"
	prodEndpoint = "http://api.pdflayer.com/api/convert?access_key=2b82226c6c03b23482e984c258866fc2&force=1&page_width=684&page_height=864"
)

var (
	ErrHTML  = errors.New("Invalid HTML")
	ErrState = errors.New("State must be 2 letter representation")

	ErrBadAddr = errors.New("Mailing address inputted is not valid. Please email engage@swayops.com with your login email / username and the address you're trying to use")
)

func ConvertHTMLToPDF(html string, res http.ResponseWriter, cfg *config.Config) error {
	if html == "" {
		return ErrHTML
	}

	form := url.Values{}
	form.Add("document_html", html)

	name := fmt.Sprintf("influencers_%s.pdf", strconv.Itoa(int(time.Now().Unix())))
	form.Add("document_name", name)

	endpoint := testEndpoint
	if !cfg.Sandbox {
		endpoint = prodEndpoint
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(res, resp.Body)

	return err
}
