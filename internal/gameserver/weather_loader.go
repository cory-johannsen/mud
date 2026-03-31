package gameserver

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// WeatherType defines a single weather event type loaded from content/weather.yaml.
type WeatherType struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Announce    string   `yaml:"announce"`
	EndAnnounce string   `yaml:"end_announce"`
	Seasons     []string `yaml:"seasons"`
	Weight      int      `yaml:"weight"`
	Conditions  []string `yaml:"conditions"`
}

type weatherFile struct {
	Types []WeatherType `yaml:"types"`
}

// LoadWeatherTypes reads and parses the weather type definitions from path.
//
// Precondition: path points to a valid weather YAML file.
// Postcondition: Returns a non-empty slice of WeatherType on success.
func LoadWeatherTypes(path string) ([]WeatherType, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("weather loader: read %q: %w", path, err)
	}
	var wf weatherFile
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("weather loader: parse %q: %w", path, err)
	}
	if len(wf.Types) == 0 {
		return nil, fmt.Errorf("weather loader: %q defines no weather types", path)
	}
	return wf.Types, nil
}

// SeasonForMonth returns the season name for a calendar month (1–12).
//
// Precondition: month in [1,12].
// Postcondition: returns one of "spring", "summer", "fall", "winter".
func SeasonForMonth(month int) string {
	switch month {
	case 3, 4, 5:
		return "spring"
	case 6, 7, 8:
		return "summer"
	case 9, 10, 11:
		return "fall"
	default: // 12, 1, 2
		return "winter"
	}
}
