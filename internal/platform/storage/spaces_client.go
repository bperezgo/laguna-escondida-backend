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
		awsconfig.WithRegion(cfg.SpacesRegion),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.SpacesKey,
			cfg.SpacesSecret,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.SpacesEndpoint)
		o.UsePathStyle = false
	})

	return &SpacesClient{
		client:   client,
		bucket:   cfg.SpacesBucket,
		endpoint: cfg.SpacesEndpoint,
	}, nil
}

func (c *SpacesClient) Upload(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
		ACL:         "private",
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
	return fmt.Sprintf("https://%s.%s/%s", c.bucket, c.endpoint, key)
}
