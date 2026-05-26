package storage

import (
	"context"
	"io"
	"net/url"
	"path"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioObjectStorage struct {
	client        *minio.Client
	bucket        string
	publicBaseURL string
}

type MinioStorage struct {
	client *minio.Client
}

func NewMinioObjectStorage(
	endpoint string,
	accessKey string,
	secretKey string,
	useSSL bool,
	bucket string,
	publicBaseURL string,
) (*MinioObjectStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}

	return &MinioObjectStorage{
		client:        client,
		bucket:        bucket,
		publicBaseURL: publicBaseURL,
	}, nil
}

func (s *MinioObjectStorage) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
}

func (s *MinioObjectStorage) BucketName() string {
	return s.bucket
}

func (s *MinioObjectStorage) PutObject(
	ctx context.Context,
	objectKey string,
	reader io.Reader,
	size int64,
	contentType string,
) error {
	_, err := s.client.PutObject(ctx, s.bucket, objectKey, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *MinioObjectStorage) RemoveObject(ctx context.Context, objectKey string) error {
	return s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{})
}

func (s *MinioObjectStorage) ObjectURL(objectKey string) string {
	u, err := url.Parse(s.publicBaseURL)
	if err != nil {
		return ""
	}
	u.Path = path.Join(u.Path, s.bucket, objectKey)
	return u.String()
}

func (s *MinioStorage) PresignedGetObject(
	ctx context.Context,
	bucket string,
	objectKey string,
	ttl time.Duration,
) (string, error) {
	presignedURL, err := s.client.PresignedGetObject(
		ctx,
		bucket,
		objectKey,
		ttl,
		url.Values{},
	)
	if err != nil {
		return "", err
	}

	return presignedURL.String(), nil
}

func (s *MinioStorage) PresignedPutObject(
	ctx context.Context,
	bucket string,
	objectKey string,
	ttl time.Duration,
	contentType string,
) (string, error) {
	query := url.Values{}

	if contentType != "" {
		query.Set("Content-Type", contentType)
	}

	presignedURL, err := s.client.PresignedPutObject(
		ctx,
		bucket,
		objectKey,
		ttl,
	)
	if err != nil {
		return "", err
	}

	return presignedURL.String(), nil
}
