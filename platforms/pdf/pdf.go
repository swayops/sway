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

func ConvertHTMLToPDF(html string, res http.ResponseWriter, cfg *config.Config) (err error) {
	if html == "" {
		err = ErrHTML
		return
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
		return
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if err != nil {
		return
	}

	_, err = io.Copy(res, resp.Body)
	if err != nil {
		return
	}

	return
}
