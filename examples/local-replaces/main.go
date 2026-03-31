package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"

	"github.com/go-overlay/examples/local-replaces/units"
)

const (
	northPoleLat = 90.0
	northPoleLon = 0.0
)

type ipLocation struct {
	City        string  `json:"city"`
	Region      string  `json:"region"`
	CountryName string  `json:"country_name"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Error       bool    `json:"error"`
}

type weatherResponse struct {
	Current struct {
		Temperature float64 `json:"temperature_2m"`
	} `json:"current"`
}

func main() {
	loc, err := lookupLocation()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error looking up location: %v\n", err)
		os.Exit(1)
	}

	celsius, err := lookupWeather(loc.Latitude, loc.Longitude)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error looking up weather: %v\n", err)
		os.Exit(1)
	}

	fahrenheit := units.CelsiusToFahrenheit(celsius)
	distKm := haversineKm(loc.Latitude, loc.Longitude, northPoleLat, northPoleLon)
	distMi := units.KilometresToMiles(distKm)

	fmt.Printf("Weather in %s, %s\n", loc.City, loc.CountryName)
	fmt.Printf("  %.1f°C  /  %.1f°F\n", celsius, fahrenheit)
	fmt.Println()
	fmt.Println("Distance to the North Pole")
	fmt.Printf("  %.0f km  /  %.0f mi\n", distKm, distMi)
}

func lookupLocation() (*ipLocation, error) {
	resp, err := http.Get("https://ipapi.co/json/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var loc ipLocation
	if err := json.NewDecoder(resp.Body).Decode(&loc); err != nil {
		return nil, err
	}
	if loc.Error {
		return nil, fmt.Errorf("IP location lookup failed")
	}
	return &loc, nil
}

func lookupWeather(lat, lon float64) (float64, error) {
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m",
		lat, lon,
	)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var w weatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		return 0, err
	}
	return w.Current.Temperature, nil
}

// haversineKm returns the great-circle distance in kilometres between two
// points on Earth given their latitude and longitude in decimal degrees.
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	lat1R := lat1 * math.Pi / 180
	lat2R := lat2 * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1R)*math.Cos(lat2R)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
