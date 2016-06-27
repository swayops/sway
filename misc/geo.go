package misc

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

const (
	GOOGLE_GEO = "http://maps.googleapis.com/maps/api/geocode/json?latlng=%s,%s"
)

type GeoRecord struct {
	City string `json:"city,omitempty"`
	// State   string `json:"state,omitempty"`
	Country string `json:"country,omitempty"`
	// Zip     string `json:"zip,omitempty"`

	Latitude   float64 `json:"lat,omitempty"`
	Longtitude float64 `json:"long,omitempty"`

	Timestamp int64 `json:"ts,omitempty"`
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

func GetGeoFromCoords(lat, long float64, ts int64) *GeoRecord {
	if lat == 0 && long == 0 {
		return nil
	}

	geo := &GeoRecord{
		Latitude:   lat,
		Longtitude: long,
		Timestamp:  ts,
	}

	var output GoogleRequest
	fLat := strconv.FormatFloat(lat, 'f', 6, 64)
	fLong := strconv.FormatFloat(long, 'f', 6, 64)
	if err := Request("GET", fmt.Sprintf(GOOGLE_GEO, fLat, fLong), "", &output); err == nil {
		for _, result := range output.Results {
			for _, val := range result.AddressComponents {
				for _, cat := range val.Types {
					// if cat == "postal_code" {
					// 	geo.Zip = val.LongName
					// } else if cat == "administrative_area_level_1" {
					// 	geo.State = val.ShortName
					if cat == "locality" {
						geo.City = val.LongName
					} else if cat == "country" {
						geo.Country = val.LongName // i.e. United States, United Kingdom
					}
				}
			}
		}
	} else {
		log.Println("Error extracting geo", err)
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

	needle.City = strings.ToLower(needle.City)
	needle.Country = strings.ToLower(needle.Country)

	// Only doing city and country for now
	for _, h := range haystack {
		ct := strings.ToLower(h.City)
		cy := strings.ToLower(h.Country)
		// Just Country Target
		if ct == "" && cy != "" {
			if cy == needle.Country {
				return true
			}
		}

		// City & Country Target
		if ct != "" && cy != "" {
			if ct == needle.City && cy == needle.Country {
				return true
			}
		}
	}

	return false
}
