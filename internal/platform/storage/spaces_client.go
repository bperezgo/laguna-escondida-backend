package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"laguna-escondida/backend/internal/platform/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type SpacesClient struct {
	client   *s3.Client
	bucket   string
	endpoint string
}

func NewSpacesClient(cfg *config.Config) (*SpacesClient, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		// DigitalOcean Spaces requires us-east-1 as the region for AWS SDK compatibility
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.SpacesKey,
			cfg.SpacesSecret,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Use path-style URLs: https://{region}.digitaloceanspaces.com/{bucket}/{key}
	endpoint := fmt.Sprintf("https://%s.digitaloceanspaces.com", cfg.SpacesRegion)

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
		// Disable request checksum calculation - DigitalOcean Spaces doesn't support it
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	return &SpacesClient{
		client:   client,
		bucket:   cfg.SpacesBucket,
		endpoint: fmt.Sprintf("%s.digitaloceanspaces.com", cfg.SpacesRegion),
	}, nil
}

func (c *SpacesClient) Upload(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to Spaces: %w", err)
	}
	return nil
}

func (c *SpacesClient) Download(ctx context.Context, key string) ([]byte, error) {
	result, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download from Spaces: %w", err)
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

func (c *SpacesClient) GetPublicURL(key string) string {
	// Path-style URL format: https://{region}.digitaloceanspaces.com/{bucket}/{key}
	return fmt.Sprintf("https://%s/%s/%s", c.endpoint, c.bucket, key)
}

func (c *SpacesClient) Delete(ctx context.Context, key string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from Spaces: %w", err)
	}
	return nil
}
