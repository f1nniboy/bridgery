package media

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type s3 struct {
	Endpoint  string `toml:"endpoint,commented" comment:"S3 host, no scheme"`
	Region    string `toml:"region,commented" comment:"bucket region"`
	Bucket    string `toml:"bucket,commented"`
	AccessKey string `toml:"access_key,commented"`
	SecretKey string `toml:"secret_key,commented"`
	BaseURL   string `toml:"base_url,commented" comment:"public base URL serving the bucket (CDN or public endpoint)\nyou make the bucket publicly readable yourself"`
	Insecure  bool   `toml:"insecure,commented" comment:"use http instead of https (local testing)"`

	client *minio.Client
}

func (s *s3) Open() error {
	if s.Endpoint == "" || s.Bucket == "" || s.BaseURL == "" {
		return errors.New("backend requires endpoint, bucket, and base_url")
	}
	client, err := minio.New(s.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s.AccessKey, s.SecretKey, ""),
		Secure: !s.Insecure,
		Region: s.Region,
	})
	if err != nil {
		return err
	}
	s.client = client
	s.BaseURL = strings.TrimRight(s.BaseURL, "/")
	return nil
}

func (s *s3) Example() any {
	return &s3{
		Endpoint: "s3.amazonaws.com",
		Region:   "us-east-1",
		Bucket:   "my-bucket",
		BaseURL:  "https://cdn.example.com",
	}
}

func (s *s3) Get(ctx context.Context, key, name string) (string, bool) {
	obj := object(key, name)
	if _, err := s.client.StatObject(ctx, s.Bucket, obj, minio.StatObjectOptions{}); err != nil {
		return "", false
	}
	return s.url(obj), true
}

func (s *s3) Put(ctx context.Context, key, name, mime string, data []byte) (string, error) {
	obj := object(key, name)
	if _, err := s.client.PutObject(ctx, s.Bucket, obj, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: mime}); err != nil {
		return "", err
	}
	return s.url(obj), nil
}

func (s *s3) url(obj string) string {
	return s.BaseURL + "/" + obj
}

func object(key, name string) string {
	return key + "/" + safeName(name)
}
