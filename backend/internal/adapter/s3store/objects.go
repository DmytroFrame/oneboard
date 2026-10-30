// Package s3store keeps files (avatars, card attachments) in S3-compatible object storage (RustFS).
package s3store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/config"
	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/attachment"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/board"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/organization"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/project"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/user"
)

var Module = fx.Module("s3",
	fx.Provide(
		NewObjectStore,
		func(s *ObjectStore) user.AvatarStorage { return s },
		func(s *ObjectStore) attachment.Storage { return s },
		func(s *ObjectStore) board.FileStorage { return s },
		func(s *ObjectStore) project.FileStorage { return s },
		func(s *ObjectStore) organization.FileStorage { return s },
	),
)

type ObjectStore struct {
	client *s3.Client
	bucket string
	region string
}

// NewObjectStore connects to RustFS locally (static keys, custom endpoint, path-style URLs) or to
// AWS S3 in production (no endpoint, credentials from the default chain: the ECS task / EC2 role).
func NewObjectStore(cfg config.Config) (*ObjectStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var awsCfg aws.Config
	if cfg.S3AccessKey != "" {
		awsCfg = aws.Config{Region: cfg.S3Region, Credentials: credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, "")}
	} else {
		var err error
		if awsCfg, err = awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.S3Region)); err != nil {
			return nil, fmt.Errorf("s3: aws credentials: %w", err)
		}
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.S3ForcePathStyle // RustFS/MinIO don't do virtual-hosted buckets on localhost
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
			// Only send/verify checksums when the API requires them — not every S3-compatible server
			// supports the SDK's newer defaults. Real S3 keeps them.
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
			o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
		}
	})
	s := &ObjectStore{client: client, bucket: cfg.S3Bucket, region: cfg.S3Region}

	if err := s.ensureBucket(ctx, cfg.S3CreateBucket); err != nil {
		return nil, fmt.Errorf("s3 bucket %q: %w", cfg.S3Bucket, err)
	}
	return s, nil
}

func (s *ObjectStore) ensureBucket(ctx context.Context, create bool) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
	if err == nil {
		return nil
	}
	var notFound *types.NotFound
	if !errors.As(err, &notFound) || !create {
		return err
	}
	in := &s3.CreateBucketInput{Bucket: &s.bucket}
	if s.region != "" && s.region != "us-east-1" { // us-east-1 rejects an explicit location
		in.CreateBucketConfiguration = &types.CreateBucketConfiguration{LocationConstraint: types.BucketLocationConstraint(s.region)}
	}
	_, err = s.client.CreateBucket(ctx, in)
	var owned *types.BucketAlreadyOwnedByYou
	if errors.As(err, &owned) {
		return nil
	}
	return err
}

func (s *ObjectStore) Put(ctx context.Context, key, contentType string, body io.ReadSeeker, size int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &s.bucket,
		Key:           &key,
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   &contentType,
	})
	return err
}

func isNoSuchKey(err error) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchKey"
}

// Open reads an object, or the byte range ("bytes=0-99") of it when byteRange is set.
func (s *ObjectStore) Open(ctx context.Context, key, byteRange string) (attachment.Object, error) {
	in := &s3.GetObjectInput{Bucket: &s.bucket, Key: &key}
	if byteRange != "" {
		in.Range = &byteRange
	}
	out, err := s.client.GetObject(ctx, in)
	if err != nil {
		var apiErr smithy.APIError
		switch {
		case isNoSuchKey(err):
			return attachment.Object{}, domain.ErrNoRows
		case errors.As(err, &apiErr) && apiErr.ErrorCode() == "InvalidRange":
			return attachment.Object{}, attachment.ErrInvalidRange
		}
		return attachment.Object{}, err
	}
	return attachment.Object{
		Body:         out.Body,
		ContentType:  aws.ToString(out.ContentType),
		Size:         aws.ToInt64(out.ContentLength),
		ContentRange: aws.ToString(out.ContentRange),
	}, nil
}

// DeleteMany removes objects in batches of 1000 (the S3 limit per request).
func (s *ObjectStore) DeleteMany(ctx context.Context, keys []string) error {
	for start := 0; start < len(keys); start += 1000 {
		batch := keys[start:min(start+1000, len(keys))]
		ids := make([]types.ObjectIdentifier, len(batch))
		for i := range batch {
			ids[i] = types.ObjectIdentifier{Key: &batch[i]}
		}
		out, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: &s.bucket,
			Delete: &types.Delete{Objects: ids, Quiet: aws.Bool(true)},
		})
		if err != nil {
			return err
		}
		if len(out.Errors) > 0 {
			return fmt.Errorf("s3: %d objects not deleted, first: %s", len(out.Errors), aws.ToString(out.Errors[0].Message))
		}
	}
	return nil
}

func (s *ObjectStore) Get(ctx context.Context, key string) (io.ReadCloser, user.ObjectInfo, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		if isNoSuchKey(err) {
			return nil, user.ObjectInfo{}, domain.ErrNoRows
		}
		return nil, user.ObjectInfo{}, err
	}
	return out.Body, user.ObjectInfo{ContentType: aws.ToString(out.ContentType), Size: aws.ToInt64(out.ContentLength)}, nil
}

func (s *ObjectStore) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	return err
}
