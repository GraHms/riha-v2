package filestore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"riha/internal/core/ports"
)

const (
	MaxObject = 16 << 20
	r2Scheme  = "r2:"
)

type R2Config struct {
	AccountID string
	AccessKey string
	SecretKey string
	Bucket    string
	Prefix    string
	Endpoint  string
}

func (c R2Config) Enabled() bool { return c.Bucket != "" && c.AccessKey != "" && c.SecretKey != "" }

func (c R2Config) endpoint() string {
	if c.Endpoint != "" {
		return strings.TrimRight(c.Endpoint, "/")
	}
	return "https://" + c.AccountID + ".r2.cloudflarestorage.com"
}

type R2 struct {
	client *s3.Client
	bucket string
	prefix string
}

func NewR2(c R2Config) (*R2, error) {
	if !c.Enabled() || (c.AccountID == "" && c.Endpoint == "") {
		return nil, errors.New("r2: account id or endpoint, bucket, access key and secret key are required")
	}
	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(c.endpoint()),
		Credentials:  credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, ""),
		UsePathStyle: true,
	})
	return &R2{client: client, bucket: c.Bucket, prefix: strings.Trim(c.Prefix, "/")}, nil
}

func clean(s string) string {
	s = path.Base("/" + s)
	if s == "/" || s == "." || s == ".." {
		return "_"
	}
	return s
}

func (r *R2) key(dir, name string) string {
	k := clean(dir) + "/" + clean(name)
	if r.prefix != "" {
		k = r.prefix + "/" + k
	}
	return k
}

func (r *R2) keyOf(p string) (string, error) {
	k, ok := strings.CutPrefix(p, r2Scheme)
	if !ok || k == "" || strings.Contains(k, "..") || (r.prefix != "" && !strings.HasPrefix(k, r.prefix+"/")) {
		return "", errors.New("r2: path outside store")
	}
	return k, nil
}

func timeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 30*time.Second)
}

func (r *R2) Save(ctx context.Context, dir, name, contentType string, body io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(body, MaxObject+1))
	if err != nil {
		return "", err
	}
	if len(data) > MaxObject {
		return "", errors.New("r2: object too large")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	k := r.key(dir, name)
	ctx, cancel := timeout(ctx)
	defer cancel()
	_, err = r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(r.bucket), Key: aws.String(k), Body: bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))), ContentType: aws.String(contentType),
		CacheControl: aws.String("private, no-store"), ContentDisposition: aws.String("attachment"),
	})
	if err != nil {
		return "", fmt.Errorf("r2 put: %w", err)
	}
	return r2Scheme + k, nil
}

func (r *R2) Read(ctx context.Context, p string) ([]byte, error) {
	k, err := r.keyOf(p)
	if err != nil {
		return nil, err
	}
	ctx, cancel := timeout(ctx)
	defer cancel()
	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(r.bucket), Key: aws.String(k)})
	if err != nil {
		return nil, fmt.Errorf("r2 get: %w", err)
	}
	defer out.Body.Close()
	data, err := io.ReadAll(io.LimitReader(out.Body, MaxObject+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxObject {
		return nil, errors.New("r2: object too large")
	}
	return data, nil
}

func (r *R2) Remove(ctx context.Context, p string) error {
	k, err := r.keyOf(p)
	if err != nil {
		return err
	}
	ctx, cancel := timeout(ctx)
	defer cancel()
	_, err = r.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(r.bucket), Key: aws.String(k)})
	return err
}

func (r *R2) Check(ctx context.Context) error {
	ctx, cancel := timeout(ctx)
	defer cancel()
	_, err := r.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(r.bucket)})
	return err
}

type Router struct {
	Primary ports.FileStore
	Remote  ports.FileStore
	Local   ports.FileStore
}

func (r Router) pick(p string) ports.FileStore {
	if strings.HasPrefix(p, r2Scheme) {
		return r.Remote
	}
	return r.Local
}

func (r Router) Save(ctx context.Context, dir, name, contentType string, body io.Reader) (string, error) {
	return r.Primary.Save(ctx, dir, name, contentType, body)
}

func (r Router) Read(ctx context.Context, p string) ([]byte, error) {
	s := r.pick(p)
	if s == nil {
		return nil, errors.New("file store not configured for " + p)
	}
	return s.Read(ctx, p)
}

func (r Router) Remove(ctx context.Context, p string) error {
	s := r.pick(p)
	if s == nil {
		return errors.New("file store not configured for " + p)
	}
	return s.Remove(ctx, p)
}
