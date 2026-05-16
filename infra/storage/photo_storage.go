package storage

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type PhotoStorage struct {
	client          *minio.Client
	originalsBucket string
	derivedBucket   string
	publicBaseURL   string
}

func NewPhotoStorage(
	endpoint string,
	accessKey string,
	secretKey string,
	useSSL bool,
	originalsBucket string,
	derivedBucket string,
	publicBaseURL string,
) (*PhotoStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}

	return &PhotoStorage{
		client:          client,
		originalsBucket: originalsBucket,
		derivedBucket:   derivedBucket,
		publicBaseURL:   publicBaseURL,
	}, nil
}

func (s *PhotoStorage) EnsureBuckets(ctx context.Context) error {
	for _, bucket := range []string{s.originalsBucket, s.derivedBucket} {
		exists, err := s.client.BucketExists(ctx, bucket)
		if err != nil {
			return err
		}
		if !exists {
			if err := s.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				return err
			}
		}
	}

	// derived bucket — public read-only
	if err := s.client.SetBucketPolicy(ctx, s.derivedBucket, buildPublicReadBucketPolicy(s.derivedBucket)); err != nil {
		return err
	}

	return nil
}

func buildPublicReadBucketPolicy(bucket string) string {
	policy := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Effect":    "Allow",
				"Principal": map[string]any{"AWS": []string{"*"}},
				"Action":    []string{"s3:GetObject"},
				"Resource":  []string{"arn:aws:s3:::" + bucket + "/*"},
			},
		},
	}

	raw, _ := json.Marshal(policy)
	return string(raw)
}

func (s *PhotoStorage) OriginalBucket() string {
	return s.originalsBucket
}

func (s *PhotoStorage) DerivedBucket() string {
	return s.derivedBucket
}

func (s *PhotoStorage) BuildOriginalObjectKey(competitionSlug string, photoID uuid.UUID, originalFilename string) string {
	ext := strings.ToLower(filepath.Ext(originalFilename))
	if ext == "" {
		ext = ".bin"
	}
	return path.Join("competitions", competitionSlug, photoID.String(), "original"+ext)
}

func (s *PhotoStorage) BuildWatermarkedObjectKey(competitionSlug string, photoID uuid.UUID) string {
	return path.Join("competitions", competitionSlug, photoID.String(), "watermarked.jpg")
}

func (s *PhotoStorage) BuildPreviewObjectKey(competitionSlug string, photoID uuid.UUID) string {
	return path.Join("competitions", competitionSlug, photoID.String(), "preview.jpg")
}

func (s *PhotoStorage) PutOriginalFromPath(ctx context.Context, objectKey, filePath, contentType string) error {
	_, err := s.client.FPutObject(ctx, s.originalsBucket, objectKey, filePath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *PhotoStorage) PutDerivedFromPath(ctx context.Context, objectKey, filePath, contentType string) error {
	_, err := s.client.FPutObject(ctx, s.derivedBucket, objectKey, filePath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *PhotoStorage) RemoveObject(ctx context.Context, bucket, objectKey string) error {
	return s.client.RemoveObject(ctx, bucket, objectKey, minio.RemoveObjectOptions{})
}

func (s *PhotoStorage) ObjectURL(bucket, objectKey string) string {
	u, err := url.Parse(s.publicBaseURL)
	if err != nil {
		return ""
	}
	u.Path = path.Join(u.Path, bucket, objectKey)
	return u.String()
}

func (s *PhotoStorage) OpenObject(ctx context.Context, bucket, objectKey string) (io.ReadCloser, string, error) {
	obj, err := s.client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", err
	}

	stat, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, "", err
	}

	return obj, stat.ContentType, nil
}
