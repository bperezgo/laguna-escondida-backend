package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"laguna-escondida/backend/internal/platform/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/cloudfront/sign"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Client struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	region        string
	endpoint      string
	cdnURL        string
	urlSigner     *sign.URLSigner
	urlTTL        time.Duration
}

func NewS3Client(cfg *config.Config) (*S3Client, error) {
	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.StorageRegion),
	}

	// Static credentials are optional: when both are set we use them, otherwise the
	// AWS SDK default credential chain (env, shared config, or the ECS/EC2 IAM role)
	// resolves credentials at request time.
	if cfg.StorageAccessKey != "" && cfg.StorageSecret != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.StorageAccessKey, cfg.StorageSecret, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	urlSigner, err := newCloudFrontSigner(cfg)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		// STORAGE_ENDPOINT overrides the default AWS S3 endpoint (e.g.
		// http://localhost:9000 for a local MinIO). S3-compatible services need
		// path-style addressing; real AWS S3 uses the default virtual-hosted style.
		if cfg.StorageEndpoint != "" {
			o.BaseEndpoint = aws.String(cfg.StorageEndpoint)
			o.UsePathStyle = true
		}
		// Only compute/validate checksums when the operation requires it, keeping
		// compatibility with S3-compatible services (e.g. MinIO) that reject the
		// newer default CRC checksums. Real AWS S3 is unaffected.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	return &S3Client{
		client:        client,
		presignClient: s3.NewPresignClient(client),
		bucket:        cfg.StorageBucket,
		region:        cfg.StorageRegion,
		endpoint:      cfg.StorageEndpoint,
		cdnURL:        cfg.CDNURL,
		urlSigner:     urlSigner,
		urlTTL:        cfg.CDNURLTTL,
	}, nil
}

// newCloudFrontSigner builds a CloudFront URL signer when a CDN URL, key pair ID, and
// private key are all configured; otherwise it returns nil so URLs stay unsigned. The
// private key comes from CDN_PRIVATE_KEY (inline PEM) or CDN_PRIVATE_KEY_PATH (a file).
func newCloudFrontSigner(cfg *config.Config) (*sign.URLSigner, error) {
	if cfg.CDNURL == "" || cfg.CDNKeyPairID == "" {
		return nil, nil
	}

	pemBytes, err := loadCDNPrivateKey(cfg)
	if err != nil {
		return nil, err
	}
	if pemBytes == nil {
		return nil, nil
	}

	privKey, err := sign.LoadPEMPrivKey(bytes.NewReader(pemBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse CloudFront private key: %w", err)
	}
	return sign.NewURLSigner(cfg.CDNKeyPairID, privKey), nil
}

func loadCDNPrivateKey(cfg *config.Config) ([]byte, error) {
	if cfg.CDNPrivateKey != "" {
		// Support single-line env values where PEM newlines were escaped as "\n".
		return []byte(strings.ReplaceAll(cfg.CDNPrivateKey, "\\n", "\n")), nil
	}
	if cfg.CDNPrivateKeyPath != "" {
		data, err := os.ReadFile(cfg.CDNPrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CloudFront private key file: %w", err)
		}
		return data, nil
	}
	return nil, nil
}

func (c *S3Client) Upload(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}
	return nil
}

func (c *S3Client) Download(ctx context.Context, key string) ([]byte, error) {
	result, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}
	defer func() {
		_ = result.Body.Close()
	}()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object body: %w", err)
	}

	return data, nil
}

// GetPublicURL returns the URL clients use to fetch an object. When CloudFront
// signing is configured (CDN_URL + key pair + private key), it returns a signed URL
// that expires after the configured TTL (default one week). Otherwise it returns an
// unsigned CDN/S3 URL, which only works if the object is publicly readable.
func (c *S3Client) GetPublicURL(key string) string {
	base := c.objectURL(key)
	if c.urlSigner != nil {
		signed, err := c.urlSigner.Sign(base, time.Now().Add(c.urlTTL))
		if err == nil {
			return signed
		}
		// Fall back to the unsigned URL. When the distribution restricts viewer
		// access, an unsigned URL fails closed (CloudFront returns 403) instead of
		// exposing the object, so a signing error never widens access.
	}
	return base
}

// objectURL builds the unsigned URL for an object: the CDN URL when configured, then
// a path-style endpoint URL (local MinIO), and finally a virtual-hosted S3 URL.
func (c *S3Client) objectURL(key string) string {
	if c.cdnURL != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(c.cdnURL, "/"), key)
	}
	if c.endpoint != "" {
		return fmt.Sprintf("%s/%s/%s", strings.TrimRight(c.endpoint, "/"), c.bucket, key)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", c.bucket, c.region, key)
}

func (c *S3Client) GetPresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error) {
	req, err := c.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiration))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	return req.URL, nil
}

func (c *S3Client) Delete(ctx context.Context, key string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}
	return nil
}
