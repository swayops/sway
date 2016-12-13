package genderize

import (
	"fmt"
	"strings"

	"github.com/swayops/sway/misc"
)

type Gender struct {
	Gender      string  `json:"gender"`
	Probability float32 `json:"probability"`
}

const (
	genderizeKey      = "09affea2f4878d449899c1ac1a29490c"
	genderizeEndpoint = "https://api.genderize.io/?apikey=%s&name=%s"
)

func GetFirstName(name string) string {
	parts := strings.Split(name, " ")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func GetGender(name string) (male, female bool) {
	if name == "" {
		return
	}

	endpoint := fmt.Sprintf(genderizeEndpoint, genderizeKey, name)

	var gender Gender
	err := misc.Request("GET", endpoint, "", &gender)
	if err != nil {
		return
	}

	if gender.Probability > 0.5 {
		switch gender.Gender {
		case "male", "Male":
			male = true
			return
		case "female", "Female":
			female = true
			return
		}
	}

	return
}
