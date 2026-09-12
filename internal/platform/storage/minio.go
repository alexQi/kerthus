package storage

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"regexp"
	"strings"
	"time"
)

type Store struct {
	client *minio.Client
	bucket string
}

func New(endpoint, accessKey, secret, bucket string, secure bool) (*Store, error) {
	if bucket == "" {
		return nil, fmt.Errorf("storage bucket is required")
	}
	client, e := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secret, ""), Secure: secure})
	if e != nil {
		return nil, e
	}
	return &Store{client, bucket}, nil
}
func (s *Store) EnsureBucket(ctx context.Context) error {
	exists, e := s.client.BucketExists(ctx, s.bucket)
	if e != nil {
		return e
	}
	if exists {
		return nil
	}
	return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
}
func (s *Store) Stat(ctx context.Context, key string) (int64, string, error) {
	info, e := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	return info.Size, info.ContentType, e
}
func (s *Store) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	_, e := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{ContentType: contentType})
	return e
}
func (s *Store) Remove(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, int64, string, time.Time, error) {
	if !strings.HasPrefix(key, "public/") {
		return nil, 0, "", time.Time{}, fmt.Errorf("invalid public object key")
	}
	return s.Open(ctx, key)
}

var objectKey = regexp.MustCompile(`^(public/[1-9][0-9]*/[1-9][0-9]*/[a-f0-9]{64}\.(png|jpg|webp)|private/[1-9][0-9]*/[1-9][0-9]*/[a-f0-9]{64})$`)

// Open requires the caller to authorize access to this generated object key.
func (s *Store) Open(ctx context.Context, key string) (io.ReadCloser, int64, string, time.Time, error) {
	if !objectKey.MatchString(key) {
		return nil, 0, "", time.Time{}, fmt.Errorf("invalid object key")
	}
	obj, e := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if e != nil {
		return nil, 0, "", time.Time{}, e
	}
	info, e := obj.Stat()
	if e != nil {
		obj.Close()
		return nil, 0, "", time.Time{}, e
	}
	return obj, info.Size, info.ContentType, info.LastModified, nil
}
