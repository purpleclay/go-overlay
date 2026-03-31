package units

// CelsiusToFahrenheit converts a temperature from Celsius to Fahrenheit.
func CelsiusToFahrenheit(c float64) float64 {
	return c*9/5 + 32
}

// KilometresToMiles converts a distance from kilometres to miles.
func KilometresToMiles(km float64) float64 {
	return km * 0.621371
}
