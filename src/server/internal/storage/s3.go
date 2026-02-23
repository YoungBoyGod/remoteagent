package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Storage 基于 AWS SDK v2 的 S3 兼容存储实现，
// 支持 MinIO / RustFS / AWS S3。
type S3Storage struct {
	client     *s3.Client
	presigner  *s3.PresignClient
	bucket     string
}

// S3Config S3 存储配置
type S3Config struct {
	Endpoint        string // S3 端点地址，MinIO/RustFS 需要指定
	Region          string // 区域
	Bucket          string // 存储桶名称
	AccessKeyID     string // 访问密钥 ID
	SecretAccessKey string // 访问密钥 Secret
	UsePathStyle    bool   // 是否使用路径风格（MinIO/RustFS 需要 true）
}

// NewS3Storage 创建 S3 存储实例，初始化时自动创建 bucket（如不存在）
func NewS3Storage(ctx context.Context, cfg S3Config) (*S3Storage, error) {
	resolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			if cfg.Endpoint != "" {
				return aws.Endpoint{
					URL:               cfg.Endpoint,
					HostnameImmutable: true,
				}, nil
			}
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		},
	)

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "",
		)),
		config.WithEndpointResolverWithOptions(resolver),
	)
	if err != nil {
		return nil, fmt.Errorf("加载 AWS 配置失败: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
	})

	stor := &S3Storage{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    cfg.Bucket,
	}

	// 自动创建 bucket
	if err := stor.ensureBucket(ctx); err != nil {
		return nil, err
	}

	log.Printf("S3 存储初始化完成: endpoint=%s bucket=%s", cfg.Endpoint, cfg.Bucket)
	return stor, nil
}

// ensureBucket 检查并创建 bucket
func (s *S3Storage) ensureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err == nil {
		return nil
	}

	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return fmt.Errorf("创建 bucket %s 失败: %w", s.bucket, err)
	}
	log.Printf("已创建 bucket: %s", s.bucket)
	return nil
}

// PutObject 上传对象
func (s *S3Storage) PutObject(ctx context.Context, key string, reader io.Reader, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
	}
	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("上传对象 %s 失败: %w", key, err)
	}
	return nil
}

// GetObject 下载对象，调用方需负责关闭返回的 ReadCloser
func (s *S3Storage) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("下载对象 %s 失败: %w", key, err)
	}
	return output.Body, nil
}

// DeleteObject 删除对象
func (s *S3Storage) DeleteObject(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("删除对象 %s 失败: %w", key, err)
	}
	return nil
}

// GetPresignedURL 生成预签名下载链接
func (s *S3Storage) GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	output, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("生成预签名 URL 失败: %w", err)
	}
	return output.URL, nil
}

// CopyObject 复制对象
func (s *S3Storage) CopyObject(ctx context.Context, srcKey, dstKey string) error {
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		CopySource: aws.String(s.bucket + "/" + srcKey),
		Key:        aws.String(dstKey),
	})
	if err != nil {
		return fmt.Errorf("复制对象 %s -> %s 失败: %w", srcKey, dstKey, err)
	}
	return nil
}

// ListObjects 按前缀列举对象
func (s *S3Storage) ListObjects(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	var objects []ObjectInfo
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("列举对象 prefix=%s 失败: %w", prefix, err)
		}
		for _, obj := range page.Contents {
			objects = append(objects, objectFromS3(obj))
		}
	}
	return objects, nil
}

// objectFromS3 将 S3 对象转换为 ObjectInfo
func objectFromS3(obj types.Object) ObjectInfo {
	info := ObjectInfo{
		Key:  aws.ToString(obj.Key),
		Size: aws.ToInt64(obj.Size),
	}
	if obj.LastModified != nil {
		info.LastModified = *obj.LastModified
	}
	if obj.ETag != nil {
		info.ETag = *obj.ETag
	}
	return info
}
