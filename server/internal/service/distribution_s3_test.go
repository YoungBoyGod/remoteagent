package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/storage"
)

type fakeS3Storage struct {
	objects []storage.ObjectInfo
	err     error
}

func (f *fakeS3Storage) PutObject(ctx context.Context, key string, reader io.Reader, contentType string) error {
	return nil
}
func (f *fakeS3Storage) GetObject(ctx context.Context, key string) (io.ReadCloser, error) { return nil, nil }
func (f *fakeS3Storage) DeleteObject(ctx context.Context, key string) error               { return nil }
func (f *fakeS3Storage) GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return "", nil
}
func (f *fakeS3Storage) CopyObject(ctx context.Context, srcKey, dstKey string) error { return nil }
func (f *fakeS3Storage) ListObjects(ctx context.Context, prefix string) ([]storage.ObjectInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.objects, nil
}

func TestListDistributionS3ObjectsPrefixWhitelist(t *testing.T) {
	svc := New(nil)
	svc.SetStorage(&fakeS3Storage{})

	_, err := svc.ListDistributionS3Objects(api.DistributionS3ListRequest{Prefix: "../../etc"})
	if !errors.Is(err, ErrInvalidPrefix) {
		t.Fatalf("expected ErrInvalidPrefix, got: %v", err)
	}
}

func TestListDistributionS3ObjectsPagination(t *testing.T) {
	now := time.Now()
	svc := New(nil)
	svc.SetStorage(&fakeS3Storage{objects: []storage.ObjectInfo{
		{Key: "releases/2026/a.zip", Size: 10, LastModified: now},
		{Key: "releases/2026/b.zip", Size: 20, LastModified: now},
		{Key: "releases/2026/c.zip", Size: 30, LastModified: now},
	}})

	first, err := svc.ListDistributionS3Objects(api.DistributionS3ListRequest{Prefix: "releases/2026/", PageSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(first.Items) != 2 || !first.HasMore || first.NextToken != "releases/2026/b.zip" {
		t.Fatalf("unexpected first page: %+v", first)
	}

	second, err := svc.ListDistributionS3Objects(api.DistributionS3ListRequest{
		Prefix: "releases/2026/", PageSize: 2, ContinuationToken: first.NextToken,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(second.Items) != 1 || second.HasMore || second.Items[0].Key != "releases/2026/c.zip" {
		t.Fatalf("unexpected second page: %+v", second)
	}
}

func TestCreateDistributionS3SourceRequiresKey(t *testing.T) {
	svc := New(nil)
	_, err := svc.CreateDistribution(api.DistributionCreateRequest{SourceType: "s3"})
	if err == nil || !strings.Contains(err.Error(), "s3_key is required") {
		t.Fatalf("expected s3_key required error, got: %v", err)
	}
}
