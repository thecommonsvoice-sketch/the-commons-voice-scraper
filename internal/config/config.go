package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load()
}

type Config struct {
	// Backend API
	APIBaseURL    string
	ReporterEmail string
	ReporterPass  string

	// Groq AI
	GroqAPIKey string

	// Pixabay (for related images)
	PixabayAPIKey string

	// Pexels (for related images)
	PexelsAPIKey string

	// Cloudinary
	CloudinaryURL string
	CloudinaryUploadPreset string

	// Scheduler
	ScheduleIntervalMinutes int
	LogLevel                string
}

func Load() *Config {
	return &Config{
		APIBaseURL:              getEnv("API_BASE_URL", "http://localhost:3000"),
		ReporterEmail:           getEnv("REPORTER_EMAIL", "scraper@commonvoice.com"),
		ReporterPass:            getEnv("REPORTER_PASSWORD", ""),
		GroqAPIKey:              getEnv("GROQ_API_KEY", ""),
		PixabayAPIKey:           getEnv("PIXABAY_API_KEY", ""),
		PexelsAPIKey:            getEnv("PEXELS_API_KEY", ""),
		CloudinaryURL:           getEnv("CLOUDINARY_URL", ""),
		CloudinaryUploadPreset:  getEnv("CLOUDINARY_UPLOAD_PRESET", "unsigned_upload"),
		ScheduleIntervalMinutes: getEnvInt("SCHEDULE_INTERVAL", 60),
		LogLevel:                getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func (c *Config) ScheduleInterval() time.Duration {
	return time.Duration(c.ScheduleIntervalMinutes) * time.Minute
}