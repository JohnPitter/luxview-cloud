package service

import (
	"net"
	"sync"

	"github.com/luxview/engine/pkg/logger"
	"github.com/oschwald/maxminddb-golang"
)

type GeoResult struct {
	Country string
	City    string
	Region  string
}

type GeoIP struct {
	reader *maxminddb.Reader
	mu     sync.RWMutex
}

func NewGeoIP(dbPath string) *GeoIP {
	log := logger.With("geoip")
	g := &GeoIP{}

	reader, err := maxminddb.Open(dbPath)
	if err != nil {
		log.Warn().Err(err).Str("path", dbPath).Msg("GeoLite2 database not found, geolocation disabled")
		return g
	}

	g.reader = reader
	log.Info().Str("path", dbPath).Msg("GeoLite2 database loaded")
	return g
}

func (g *GeoIP) Lookup(ipStr string) GeoResult {
	if g.reader == nil {
		return GeoResult{}
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return GeoResult{}
	}

	var record struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
		City struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"city"`
		Subdivisions []struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"subdivisions"`
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	if err := g.reader.Lookup(ip, &record); err != nil {
		return GeoResult{}
	}

	result := GeoResult{Country: record.Country.ISOCode}
	if name, ok := record.City.Names["en"]; ok {
		result.City = name
	}
	if len(record.Subdivisions) > 0 {
		if name, ok := record.Subdivisions[0].Names["en"]; ok {
			result.Region = name
		}
	}
	return result
}

func (g *GeoIP) Close() {
	if g.reader != nil {
		g.reader.Close()
	}
}
