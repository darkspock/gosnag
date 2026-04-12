package upload

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Storage abstracts file storage (local disk or S3).
type Storage interface {
	Put(ctx context.Context, filename string, contentType string, body io.Reader) (url string, err error)
}

// LocalStorage stores files on local disk.
type LocalStorage struct {
	Dir     string
	BaseURL string // e.g., "http://localhost:8080"
}

func (l *LocalStorage) Put(_ context.Context, filename, contentType string, body io.Reader) (string, error) {
	os.MkdirAll(l.Dir, 0755)
	dst, err := os.Create(filepath.Join(l.Dir, filename))
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, body); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	return l.BaseURL + "/uploads/" + filename, nil
}

// S3Storage stores files in an S3 bucket.
type S3Storage struct {
	client *s3.Client
	bucket string
	prefix string   // key prefix, e.g., "uploads/"
	cdnURL string   // public URL prefix, e.g., "https://cdn.example.com/uploads"
}

// S3Config holds S3 configuration from environment variables.
type S3Config struct {
	Bucket    string // UPLOAD_S3_BUCKET
	Region    string // UPLOAD_S3_REGION (or AWS_REGION)
	Prefix    string // UPLOAD_S3_PREFIX (default: "uploads/")
	CDNURL    string // UPLOAD_S3_CDN_URL (public URL prefix)
	AccessKey string // AWS_ACCESS_KEY_ID (optional, uses default chain if empty)
	SecretKey string // AWS_SECRET_ACCESS_KEY
}

func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	var opts []func(*awsconfig.LoadOptions) error

	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "uploads/"
	}

	return &S3Storage{
		client: s3.NewFromConfig(awsCfg),
		bucket: cfg.Bucket,
		prefix: prefix,
		cdnURL: cfg.CDNURL,
	}, nil
}

func (s *S3Storage) Put(ctx context.Context, filename, contentType string, body io.Reader) (string, error) {
	key := s.prefix + filename

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(s.bucket),
		Key:          aws.String(key),
		Body:         body,
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	if s.cdnURL != "" {
		return s.cdnURL + "/" + filename, nil
	}
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.bucket, key), nil
}
