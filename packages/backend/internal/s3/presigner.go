package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type Config struct {
	Bucket         string
	Region         string
	Endpoint       string
	AccessKey      string
	SecretKey      string
	ForcePathStyle bool
}

type Presigner struct {
	bucket  string
	client  *awss3.Client
	presign *awss3.PresignClient
}

func NewPresigner(ctx context.Context, cfg Config) (*Presigner, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	options := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		options = append(options, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	client := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		o.UsePathStyle = cfg.ForcePathStyle
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})
	return &Presigner{
		bucket:  cfg.Bucket,
		client:  client,
		presign: awss3.NewPresignClient(client),
	}, nil
}

func (p *Presigner) PresignPut(ctx context.Context, key, contentType string, size int64, expiry time.Duration) (string, map[string]string, error) {
	input := &awss3.PutObjectInput{
		Bucket:      aws.String(p.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}
	request, err := p.presign.PresignPutObject(ctx, input, func(o *awss3.PresignOptions) {
		o.Expires = expiry
	})
	if err != nil {
		return "", nil, fmt.Errorf("presign put: %w", err)
	}
	return request.URL, map[string]string{"Content-Type": contentType}, nil
}

func (p *Presigner) Put(ctx context.Context, key, contentType string, size int64, body io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(body, size+1))
	if err != nil {
		return fmt.Errorf("read upload body: %w", err)
	}
	if int64(len(data)) != size {
		return fmt.Errorf("upload body size mismatch: got %d want %d", len(data), size)
	}
	_, err = p.client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:        aws.String(p.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
		Body:          bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

func (p *Presigner) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	input := &awss3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}
	request, err := p.presign.PresignGetObject(ctx, input, func(o *awss3.PresignOptions) {
		o.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("presign get: %w", err)
	}
	return request.URL, nil
}

func (p *Presigner) Head(ctx context.Context, key string) (int64, string, error) {
	out, err := p.client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, "", fmt.Errorf("head object: %w", err)
	}
	var size int64
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	var contentType string
	if out.ContentType != nil {
		contentType = *out.ContentType
	}
	return size, contentType, nil
}
