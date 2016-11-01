package geo

import (
	"log"
	"net"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

func NewGeoDB(file string) (*maxminddb.Reader, error) {
	db, err := maxminddb.Open(file)
	if err != nil {
		return nil, err
	}
	return db, nil
}

type MaxmindRecord struct {
	// City struct {
	// 	Names map[string]string `maxminddb:"names"`
	// } `maxminddb:"city"`
	// Location struct {
	// 	Latitude  float64 `maxminddb:"latitude"`
	// 	Longitude float64 `maxminddb:"longitude"`
	// 	MetroCode uint    `maxminddb:"metro_code"`
	// } `maxminddb:"location"`
	// Postal struct {
	// 	Code string `maxminddb:"code"`
	// } `maxminddb:"postal"`
	State []struct {
		IsoCode string `maxminddb:"iso_code"`
	} `maxminddb:"subdivisions"`
	// Continent struct {
	// 	Names map[string]string `maxminddb:"names"`
	// } `maxminddb:"continent"`
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
}

func GetGeoFromIP(geoDb *maxminddb.Reader, ip string) *GeoRecord {
	if geoDb == nil || len(ip) == 0 {
		return nil
	}

	parseIp := net.ParseIP(ip)
	if parseIp == nil {
		return nil
	}

	var record MaxmindRecord
	err := geoDb.Lookup(parseIp, &record)
	if err != nil {
		return nil
	}

	if len(record.State) == 0 {
		return nil
	}

	state := strings.ToLower(record.State[0].IsoCode)
	if state == "" {
		log.Println("Failed Geo IP State lookup", err)
		return nil
	}

	_, usOK := US_STATES[state]
	_, caOK := CA_PROVINCES[state]

	if !usOK && !caOK {
		log.Println("State did not match on IP lookup", state)
		return nil
	}

	country := strings.ToLower(record.Country.ISOCode)
	if country == "" {
		log.Println("Failed Geo IP Country lookup", err)
		return nil
	}

	_, ok := COUNTRIES[country]
	if !ok {
		log.Println("Country did not match on IP lookup", country)
		return nil
	}

	g := &GeoRecord{
		Timestamp: int32(time.Now().Unix()),
		State:     state,
		Country:   country,
		Source:    "ip",
	}

	if !IsValidGeo(g) {
		log.Println("Invalid geo generated via IP")
		return nil
	}

	return g
}
