package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Client struct {
	latitude  float64
	longitude float64
	client    *http.Client
}

type WeatherData struct {
	Location Location `json:"location"`
	Current  Current  `json:"current"`
	Hourly   []Hourly `json:"hourly"`
	Daily    []Daily  `json:"daily"`
}

type Location struct {
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Timezone string  `json:"timezone"`
	Name     string  `json:"name,omitempty"`
}

type Current struct {
	Temperature         float64 `json:"temperature"`
	ApparentTemperature float64 `json:"apparent_temperature"`
	Humidity            int     `json:"humidity"`
	WindSpeed           float64 `json:"wind_speed"`
	WindDirection       int     `json:"wind_direction"`
	WeatherCode         int     `json:"weather_code"`
	WeatherDescription  string  `json:"weather_description"`
	IsDay               bool    `json:"is_day"`
	UVIndex             float64 `json:"uv_index"`
	Precipitation       float64 `json:"precipitation"`
}

type Hourly struct {
	Time               string  `json:"time"`
	Temperature        float64 `json:"temperature"`
	WeatherCode        int     `json:"weather_code"`
	WeatherDescription string  `json:"weather_description"`
	Precipitation      float64 `json:"precipitation"`
}

type Daily struct {
	Date               string  `json:"date"`
	TemperatureMax     float64 `json:"temperature_max"`
	TemperatureMin     float64 `json:"temperature_min"`
	WeatherCode        int     `json:"weather_code"`
	WeatherDescription string  `json:"weather_description"`
	PrecipitationSum   float64 `json:"precipitation_sum"`
	Sunrise            string  `json:"sunrise"`
	Sunset             string  `json:"sunset"`
}

func NewClient(latitude, longitude float64) *Client {
	return &Client{
		latitude:  latitude,
		longitude: longitude,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) GetWeather() (*WeatherData, error) {
	if c.latitude == 0 && c.longitude == 0 {
		lat, lon, err := detectLocation()
		if err != nil {
			slog.Warn("failed to detect location, using defaults", "error", err)
			c.latitude = 52.52
			c.longitude = 13.41
		} else {
			c.latitude = lat
			c.longitude = lon
		}
	}

	return c.fetchWeather()
}

func detectLocation() (float64, float64, error) {
	req, err := http.NewRequest("GET", "http://ip-api.com/json/", nil)
	if err != nil {
		return 0, 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, err
	}

	return result.Lat, result.Lon, nil
}

func (c *Client) fetchWeather() (*WeatherData, error) {
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,apparent_temperature,relative_humidity_2m,precipitation,weather_code,wind_speed_10m,wind_direction_10m,is_day,uv_index&hourly=temperature_2m,weather_code,precipitation_probability&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_sum,sunrise,sunset&timezone=auto&forecast_days=7",
		c.latitude, c.longitude,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch weather: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var raw struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Timezone  string  `json:"timezone"`
		Current   struct {
			Temperature         float64 `json:"temperature_2m"`
			ApparentTemperature float64 `json:"apparent_temperature"`
			Humidity            int     `json:"relative_humidity_2m"`
			Precipitation       float64 `json:"precipitation"`
			WeatherCode         int     `json:"weather_code"`
			WindSpeed           float64 `json:"wind_speed_10m"`
			WindDirection       int     `json:"wind_direction_10m"`
			IsDay               int     `json:"is_day"`
			UVIndex             float64 `json:"uv_index"`
		} `json:"current"`
		Hourly struct {
			Time          []string  `json:"time"`
			Temperature   []float64 `json:"temperature_2m"`
			WeatherCode   []int     `json:"weather_code"`
			Precipitation []float64 `json:"precipitation_probability"`
		} `json:"hourly"`
		Daily struct {
			Time             []string  `json:"time"`
			TemperatureMax   []float64 `json:"temperature_2m_max"`
			TemperatureMin   []float64 `json:"temperature_2m_min"`
			WeatherCode      []int     `json:"weather_code"`
			PrecipitationSum []float64 `json:"precipitation_sum"`
			Sunrise          []string  `json:"sunrise"`
			Sunset           []string  `json:"sunset"`
		} `json:"daily"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse weather response: %w", err)
	}

	weather := &WeatherData{
		Location: Location{
			Lat:      raw.Latitude,
			Lon:      raw.Longitude,
			Timezone: raw.Timezone,
		},
		Current: Current{
			Temperature:         raw.Current.Temperature,
			ApparentTemperature: raw.Current.ApparentTemperature,
			Humidity:            raw.Current.Humidity,
			WindSpeed:           raw.Current.WindSpeed,
			WindDirection:       raw.Current.WindDirection,
			WeatherCode:         raw.Current.WeatherCode,
			WeatherDescription:  getWeatherDescription(raw.Current.WeatherCode),
			IsDay:               raw.Current.IsDay == 1,
			UVIndex:             raw.Current.UVIndex,
			Precipitation:       raw.Current.Precipitation,
		},
	}

	for i := 0; i < len(raw.Hourly.Time) && i < 24; i++ {
		weather.Hourly = append(weather.Hourly, Hourly{
			Time:               raw.Hourly.Time[i],
			Temperature:        raw.Hourly.Temperature[i],
			WeatherCode:        raw.Hourly.WeatherCode[i],
			WeatherDescription: getWeatherDescription(raw.Hourly.WeatherCode[i]),
			Precipitation:      raw.Hourly.Precipitation[i],
		})
	}

	for i := 0; i < len(raw.Daily.Time); i++ {
		weather.Daily = append(weather.Daily, Daily{
			Date:               raw.Daily.Time[i],
			TemperatureMax:     raw.Daily.TemperatureMax[i],
			TemperatureMin:     raw.Daily.TemperatureMin[i],
			WeatherCode:        raw.Daily.WeatherCode[i],
			WeatherDescription: getWeatherDescription(raw.Daily.WeatherCode[i]),
			PrecipitationSum:   raw.Daily.PrecipitationSum[i],
			Sunrise:            raw.Daily.Sunrise[i],
			Sunset:             raw.Daily.Sunset[i],
		})
	}

	return weather, nil
}

func getWeatherDescription(code int) string {
	descriptions := map[int]string{
		0:  "Clear sky",
		1:  "Mainly clear",
		2:  "Partly cloudy",
		3:  "Overcast",
		45: "Fog",
		48: "Depositing rime fog",
		51: "Light drizzle",
		53: "Moderate drizzle",
		55: "Dense drizzle",
		56: "Light freezing drizzle",
		57: "Dense freezing drizzle",
		61: "Slight rain",
		63: "Moderate rain",
		65: "Heavy rain",
		66: "Light freezing rain",
		67: "Heavy freezing rain",
		71: "Slight snow fall",
		73: "Moderate snow fall",
		75: "Heavy snow fall",
		77: "Snow grains",
		80: "Slight rain showers",
		81: "Moderate rain showers",
		82: "Violent rain showers",
		85: "Slight snow showers",
		86: "Heavy snow showers",
		95: "Thunderstorm",
		96: "Thunderstorm with slight hail",
		99: "Thunderstorm with heavy hail",
	}

	if desc, ok := descriptions[code]; ok {
		return desc
	}

	return "Unknown"
}
