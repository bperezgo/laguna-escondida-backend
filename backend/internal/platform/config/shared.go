package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nyaruka/phonenumbers"
)

// Mode determines how the backend runs: as the cloud aggregate or as a
// restaurant edge node. Each binary (cmd/cloud, cmd/edge) passes its own.
type Mode string

const (
	ModeCloud Mode = "cloud"
	ModeEdge  Mode = "edge"
)

// Shared is the configuration both binaries read.
type Shared struct {
	AdminAPIKey string
	JWTSecret   string
	// NodeSyncKey authenticates edge nodes to the cloud sync endpoints: the cloud checks it,
	// the edge sends it. When unset NodeAuthMiddleware fails closed and rejects every request.
	NodeSyncKey string
	// PhoneHomeRegion (ISO 3166 alpha-2) reads customer phones typed without "+".
	PhoneHomeRegion string

	Storage       Storage
	Observability Observability
}

type Storage struct {
	Region string
	// Endpoint overrides the AWS S3 default endpoint (e.g. a local MinIO); empty for real AWS S3.
	Endpoint string
	// Credentials are optional: when unset, the AWS SDK default chain (env, shared config,
	// ECS/EC2 IAM role) is used, so the app can run under a task role without static keys.
	AccessKey string
	Secret    string
	Bucket    string
	// CDNURL is the CloudFront base URL for public object URLs; empty falls back to a
	// direct endpoint/S3 URL (useful for local dev).
	CDNURL string
	// With a key pair id plus a private key (inline PEM or a path), object URLs are signed and
	// expire after CDNURLTTL; otherwise they're unsigned, which only works for a public bucket.
	CDNKeyPairID      string
	CDNPrivateKey     string
	CDNPrivateKeyPath string
	CDNURLTTL         time.Duration
}

// Observability: when Enabled is false (default) the OTLP exporters are not started, so local
// dev boots with no collector present. The /metrics endpoint is cheap and always served.
type Observability struct {
	Enabled          bool
	ServiceName      string  // OTEL_SERVICE_NAME, default "laguna-backend"
	ServiceVersion   string  // image tag / build; default "dev"
	Environment      string  // deployment.environment; default "dev"
	OTLPEndpoint     string  // e.g. "http://127.0.0.1:4317"; empty => exporters off
	TraceSampleRatio float64 // 1.0 dev; lower in prod
	MetricsPort      string  // internal scrape port, default "9090"
}

func loadShared() (Shared, error) {
	adminAPIKey := os.Getenv("ADMIN_API_KEY")
	if adminAPIKey == "" {
		return Shared{}, errors.New("ADMIN_API_KEY is not set")
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return Shared{}, errors.New("JWT_SECRET is not set")
	}

	storage, err := loadStorage()
	if err != nil {
		return Shared{}, err
	}
	observability, err := loadObservability()
	if err != nil {
		return Shared{}, err
	}

	phoneHomeRegion := strings.ToUpper(os.Getenv("PHONE_HOME_REGION"))
	if phoneHomeRegion == "" {
		phoneHomeRegion = "CO"
	}
	if phonenumbers.GetCountryCodeForRegion(phoneHomeRegion) == 0 {
		return Shared{}, fmt.Errorf("invalid PHONE_HOME_REGION %q: not a supported ISO 3166 alpha-2 region", phoneHomeRegion)
	}

	return Shared{
		AdminAPIKey:     adminAPIKey,
		JWTSecret:       jwtSecret,
		NodeSyncKey:     os.Getenv("NODE_SYNC_KEY"),
		PhoneHomeRegion: phoneHomeRegion,
		Storage:         storage,
		Observability:   observability,
	}, nil
}

func loadStorage() (Storage, error) {
	region := os.Getenv("STORAGE_REGION")
	if region == "" {
		region = "us-east-1"
	}

	bucket := os.Getenv("STORAGE_BUCKET")
	if bucket == "" {
		return Storage{}, errors.New("STORAGE_BUCKET is not set")
	}

	cdnURLTTL := 7 * 24 * time.Hour
	if v := os.Getenv("CDN_URL_TTL"); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return Storage{}, fmt.Errorf("invalid CDN_URL_TTL %q: %w", v, err)
		}
		cdnURLTTL = parsed
	}

	return Storage{
		Region:            region,
		Endpoint:          os.Getenv("STORAGE_ENDPOINT"),
		AccessKey:         os.Getenv("STORAGE_ACCESS_KEY"),
		Secret:            os.Getenv("STORAGE_SECRET"),
		Bucket:            bucket,
		CDNURL:            os.Getenv("CDN_URL"),
		CDNKeyPairID:      os.Getenv("CDN_KEY_PAIR_ID"),
		CDNPrivateKey:     os.Getenv("CDN_PRIVATE_KEY"),
		CDNPrivateKeyPath: os.Getenv("CDN_PRIVATE_KEY_PATH"),
		CDNURLTTL:         cdnURLTTL,
	}, nil
}

func loadObservability() (Observability, error) {
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "laguna-backend"
	}
	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "dev"
	}
	serviceVersion := os.Getenv("SERVICE_VERSION")
	if serviceVersion == "" {
		serviceVersion = "dev"
	}

	traceSampleRatio := 1.0
	if v := os.Getenv("OTEL_TRACES_SAMPLER_ARG"); v != "" {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Observability{}, fmt.Errorf("invalid OTEL_TRACES_SAMPLER_ARG %q: %w", v, err)
		}
		traceSampleRatio = parsed
	}

	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9090"
	}

	return Observability{
		Enabled:          os.Getenv("OBSERVABILITY_ENABLED") == "true",
		ServiceName:      serviceName,
		ServiceVersion:   serviceVersion,
		Environment:      environment,
		OTLPEndpoint:     os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		TraceSampleRatio: traceSampleRatio,
		MetricsPort:      metricsPort,
	}, nil
}
