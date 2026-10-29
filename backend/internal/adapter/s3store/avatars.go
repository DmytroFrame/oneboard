// Package s3store keeps avatars in S3-compatible object storage (RustFS).
package s3store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"go.uber.org/fx"

	"clodapp/backend/internal/config"
	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase/user"
)

var Module = fx.Module("s3",
	fx.Provide(
		NewAvatarStorage,
		func(s *AvatarStorage) user.AvatarStorage { return s },
	),
)

type AvatarStorage struct {
	client *s3.Client
	bucket string
}

func NewAvatarStorage(cfg config.Config) (*AvatarStorage, error) {
	client := s3.New(s3.Options{
		Region:       cfg.S3Region,
		BaseEndpoint: aws.String(cfg.S3Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		UsePathStyle: true, // RustFS/MinIO don't do virtual-hosted buckets on localhost
		// Only send/verify checksums when the API requires them — not every S3-compatible server supports the new defaults.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	s := &AvatarStorage{client: client, bucket: cfg.S3Bucket}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := s.ensureBucket(ctx); err != nil {
		return nil, fmt.Errorf("s3: %w", err)
	}
	return s, nil
}

func (s *AvatarStorage) ensureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
	if err == nil {
		return nil
	}
	var notFound *types.NotFound
	if !errors.As(err, &notFound) {
		return err
	}
	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &s.bucket})
	var owned *types.BucketAlreadyOwnedByYou
	if errors.As(err, &owned) {
		return nil
	}
	return err
}

func (s *AvatarStorage) Put(ctx context.Context, key, contentType string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &s.bucket,
		Key:           &key,
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   &contentType,
	})
	return err
}

func (s *AvatarStorage) Get(ctx context.Context, key string) (io.ReadCloser, user.ObjectInfo, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchKey" {
			return nil, user.ObjectInfo{}, domain.ErrNoRows
		}
		return nil, user.ObjectInfo{}, err
	}
	return out.Body, user.ObjectInfo{ContentType: aws.ToString(out.ContentType), Size: aws.ToInt64(out.ContentLength)}, nil
}

func (s *AvatarStorage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	return err
}
