package geo

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/swayops/sway/misc"
)

const (
	GOOGLE_GEO = "https://maps.googleapis.com/maps/api/geocode/json?latlng=%s,%s&key=AIzaSyD4NwxB_AUVr3eHJt2aRxbm778DypmSwHE"
)

type GeoRecord struct {
	State   string `json:"state,omitempty"`   // ISO
	Country string `json:"country,omitempty"` // ISO

	Timestamp int32  `json:"ts,omitempty"`
	Source    string `json:"source,omitempty"`
}

type GoogleRequest struct {
	Results []struct {
		AddressComponents []struct {
			LongName  string   `json:"long_name"`
			ShortName string   `json:"short_name"`
			Types     []string `json:"types"`
		} `json:"address_components"`
	} `json:"results"`
	Status string `json:"status"`
}

func GetGeoFromCoords(lat, long float64, ts int32) *GeoRecord {
	if lat == 0 && long == 0 {
		return nil
	}

	geo := &GeoRecord{
		Timestamp: ts,
		Source:    "coords",
	}

	var output GoogleRequest
	fLat := strconv.FormatFloat(lat, 'f', 6, 64)
	fLong := strconv.FormatFloat(long, 'f', 6, 64)
	if err := misc.Request("GET", fmt.Sprintf(GOOGLE_GEO, fLat, fLong), "", &output); err == nil || len(output.Results) == 0 {
		for _, result := range output.Results {
			for _, val := range result.AddressComponents {
				for _, cat := range val.Types {
					// if cat == "postal_code" {
					// 	geo.Zip = val.LongName
					if cat == "administrative_area_level_1" {
						geo.State = val.ShortName
						// } else if cat == "locality" {
						// 	geo.City = val.LongName
					} else if cat == "country" {
						geo.Country = val.ShortName
					}
				}
			}
		}
	} else {
		log.Println("Error extracting geo", err)
	}

	if geo.State == "" && geo.Country == "" {
		return nil
	}

	if !IsValidGeo(geo) {
		log.Println("Google returned invalid geo!")
		return nil
	}

	return geo
}

func IsGeoMatch(haystack []*GeoRecord, needle *GeoRecord) bool {
	if len(haystack) == 0 {
		// This campaign does not have any geo targets.. APPROVE!
		return true
	}

	if needle == nil {
		// The campaign has geo targets but the influencer has no geo.. REJECT!
		return false
	}

	// Only doing state and country for now
	for _, h := range haystack {
		// Just Country Target
		if h.State == "" && h.Country != "" {
			if strings.EqualFold(needle.Country, h.Country) {
				return true
			}
		}

		// State & Country Target
		if h.State != "" && h.Country != "" {
			if strings.EqualFold(needle.State, h.State) && strings.EqualFold(needle.Country, h.Country) {
				return true
			}
		}
	}

	return false
}

func IsValidGeo(r *GeoRecord) bool {
	if r.Country == "" {
		return false
	}

	cy := strings.ToLower(r.Country)
	_, ok := COUNTRIES[cy]
	if !ok {
		return false
	}

	state := strings.ToLower(r.State)
	switch cy {
	case "us":
		// USA
		if state != "" {
			_, ok := US_STATES[state]
			if !ok {
				return false
			}
		}
		return true

	case "ca":
		// Canada
		if state != "" {
			_, ok := CA_PROVINCES[state]
			if !ok {
				return false
			}
		}
		return true
	default:
		// Empty out state for non-US/CA country
		r.State = ""
		return true
	}
}

func IsValidGeoTarget(r *GeoRecord) bool {
	if !IsValidGeo(r) {
		return false
	}

	cy := strings.ToLower(r.Country)
	if cy != "us" && cy != "ca" {
		if r.State != "" {
			// Only allow state targeting for US and CA
			return false
		}
	}

	return true
}
