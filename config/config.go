package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv          string
	HTTPAddr        string
	DatabaseURL     string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	MinIOEndpoint                string
	MinIOUseSSL                  bool
	MinIOAccessKey               string
	MinIOSecretKey               string
	MinIOCompetitionCoversBucket string
	MinIOPublicBaseURL           string

	MinIOPhotoOriginalsBucket string
	MinIOPhotoDerivedBucket   string

	PhotoPreviewMaxWidth  int
	PhotoPreviewMaxHeight int
	PhotoWatermarkText    string
	PhotoJPEGQuality      int

	OCRServiceURL           string
	OCRServiceTimeout       time.Duration
	PhotoWatermarkImagePath string
	CORSAllowOrigins        []string

	PhotoProcessorMode              string
	ImageProcessingServiceURL       string
	ImageProcessingServiceTimeout   time.Duration
	ImageProcessingHealthTimeout    time.Duration
	ImageProcessingBatchSize        int
	ImageProcessingPresignedTTL     time.Duration
	ImageProcessingWatermarkOpacity float64
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	PhotoUploadAsync         bool
	PhotoUploadChunkMaxFiles int
	PhotoUploadStatusTTL     time.Duration

	ImageProcessingWorkerEnabled       bool
	ImageProcessingWorkerBatchSize     int
	ImageProcessingWorkerFlushInterval time.Duration
	ImageProcessingWorkerConcurrency   int
}

func MustLoad() Config {
	return Config{
		AppEnv:          getEnv("APP_ENV", "dev"),
		HTTPAddr:        getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:     mustGetEnv("DATABASE_URL"),
		JWTSecret:       mustGetEnv("JWT_SECRET"),
		AccessTokenTTL:  getEnvDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: getEnvDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour),

		MinIOEndpoint:                mustGetEnv("MINIO_ENDPOINT"),
		MinIOUseSSL:                  getEnvBool("MINIO_USE_SSL", false),
		MinIOAccessKey:               mustGetEnv("MINIO_ACCESS_KEY"),
		MinIOSecretKey:               mustGetEnv("MINIO_SECRET_KEY"),
		MinIOCompetitionCoversBucket: mustGetEnv("MINIO_COMPETITION_COVERS_BUCKET"),
		MinIOPublicBaseURL:           mustGetEnv("MINIO_PUBLIC_BASE_URL"),

		MinIOPhotoOriginalsBucket: mustGetEnv("MINIO_PHOTO_ORIGINALS_BUCKET"),
		MinIOPhotoDerivedBucket:   mustGetEnv("MINIO_PHOTO_DERIVED_BUCKET"),

		PhotoPreviewMaxWidth:  getEnvInt("PHOTO_PREVIEW_MAX_WIDTH", 1600),
		PhotoPreviewMaxHeight: getEnvInt("PHOTO_PREVIEW_MAX_HEIGHT", 1600),
		PhotoWatermarkText:    getEnv("PHOTO_WATERMARK_TEXT", "Photo Service"),
		PhotoJPEGQuality:      getEnvInt("PHOTO_JPEG_QUALITY", 85),

		OCRServiceURL:           getEnv("OCR_SERVICE_URL", ""),
		OCRServiceTimeout:       mustParseDuration(getEnv("OCR_SERVICE_TIMEOUT", "15s")),
		PhotoWatermarkImagePath: getEnv("PHOTO_WATERMARK_IMAGE_PATH", ""),
		CORSAllowOrigins:        splitCSV(getEnv("CORS_ALLOW_ORIGINS", "http://localhost:5173")),

		PhotoProcessorMode:              getEnv("PHOTO_PROCESSOR_MODE", "auto"),
		ImageProcessingServiceURL:       getEnv("IMAGE_PROCESSING_SERVICE_URL", "http://image_processing_service:8081"),
		ImageProcessingServiceTimeout:   getEnvDuration("IMAGE_PROCESSING_SERVICE_TIMEOUT", 120*time.Second),
		ImageProcessingHealthTimeout:    getEnvDuration("IMAGE_PROCESSING_HEALTH_TIMEOUT", 2*time.Second),
		ImageProcessingBatchSize:        getEnvInt("IMAGE_PROCESSING_BATCH_SIZE", 25),
		ImageProcessingPresignedTTL:     getEnvDuration("IMAGE_PROCESSING_PRESIGNED_URL_TTL", 10*time.Minute),
		ImageProcessingWatermarkOpacity: getEnvFloat("IMAGE_PROCESSING_WATERMARK_OPACITY", 1),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		PhotoUploadAsync:         getEnvBool("PHOTO_UPLOAD_ASYNC", false),
		PhotoUploadChunkMaxFiles: getEnvInt("PHOTO_UPLOAD_CHUNK_MAX_FILES", 200),
		PhotoUploadStatusTTL:     getEnvDuration("PHOTO_UPLOAD_STATUS_TTL", 24*time.Hour),

		ImageProcessingWorkerEnabled:       getEnvBool("IMAGE_PROCESSING_WORKER_ENABLED", false),
		ImageProcessingWorkerBatchSize:     getEnvInt("IMAGE_PROCESSING_WORKER_BATCH_SIZE", 500),
		ImageProcessingWorkerFlushInterval: getEnvDuration("IMAGE_PROCESSING_WORKER_FLUSH_INTERVAL", 2*time.Second),
		ImageProcessingWorkerConcurrency:   getEnvInt("IMAGE_PROCESSING_WORKER_CONCURRENCY", 1),
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))

	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v != "" {
			out = append(out, v)
		}
	}

	return out
}

func mustParseDuration(value string) time.Duration {
	d, err := time.ParseDuration(value)
	if err != nil {
		log.Fatalf("invalid duration: %s", value)
	}
	return d
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env: %s", key)
	}
	return v
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		log.Fatalf("invalid duration in %s: %v", key, err)
	}
	return d
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		log.Fatalf("invalid bool in %s: %v", key, err)
	}
	return b
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("invalid int in %s: %v", key, err)
	}
	return i
}

func getEnvFloat(key string, defaultValue float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return defaultValue
	}

	return parsed
}
