package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Builder-Lawyers/builder-backend/pkg/env"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Storage struct {
	client *s3.Client
	bucket string
	region string
}

func NewStorage(config aws.Config) *Storage {
	return &Storage{
		initClient(config),
		env.GetEnv("S3_BUCKET", "sanity-web"),
		env.GetEnv("AWS_DEFAULT_REGION", "eu-north-1"),
	}
}

func initClient(config aws.Config) *s3.Client {
	client := s3.NewFromConfig(config, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	return client
}

func (s *Storage) UploadFile(ctx context.Context, key string, contentType *string, body io.Reader) (string, error) {
	var ct string

	data, err := io.ReadAll(body)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("reading for content-type detection: %v", err)
	}

	if contentType == nil {
		ct = http.DetectContentType(data)
		if strings.HasSuffix(key, ".svg") {
			ct = "image/svg+xml"
		}
		if strings.HasSuffix(key, ".css") {
			ct = "text/css"
		}
	} else {
		ct = *contentType
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentType:   aws.String(ct),
		ContentLength: aws.Int64(int64(len(data))),
	})
	if err != nil {
		return "", err
	}

	fileURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
	return fileURL, nil
}

func (s *Storage) ListFiles(ctx context.Context, limit int32, input *s3.ListObjectsV2Input) []string {
	input.Bucket = &s.bucket

	p := s3.NewListObjectsV2Paginator(s.client, input, func(o *s3.ListObjectsV2PaginatorOptions) {
		o.Limit = limit
	})

	var i int
	var files []string
	for p.HasMorePages() {
		i++
		page, err := p.NextPage(ctx)
		if err != nil {
			slog.Error("failed to get page", "err", err)
		}
		for _, obj := range page.Contents {
			files = append(files, *obj.Key)
		}
	}
	return files
}

func (s *Storage) GetFile(ctx context.Context, key string) ([]byte, error) {
	params := &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(key),
	}
	resp, err := s.client.GetObject(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("error downloading file %v: %v", key, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading file contents, %v", err)
	}

	return data, nil
}

func (s *Storage) DownloadFiles(ctx context.Context, keys []string, destination, pathAfter string) error {
	for _, key := range keys {
		params := &s3.GetObjectInput{
			Bucket: &s.bucket,
			Key:    aws.String(key),
		}
		resp, err := s.client.GetObject(ctx, params)
		if err != nil {
			return fmt.Errorf("error downloading key %s: %w", key, err)
		}
		destKey := strings.TrimPrefix(key, pathAfter)
		//slog.Info("got object from s3, uploading to local",
		//	"key", key,
		//	"destination", filepath.Join(destination, destKey),
		//)

		err = s.readAndCopyObjectTo(resp.Body, filepath.Join(destination, destKey))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) readAndCopyObjectTo(content io.ReadCloser, destination string) error {
	//slog.Info("saving file to", "dest", destination)
	if err := os.MkdirAll(filepath.Dir(destination), os.ModePerm); err != nil {
		return fmt.Errorf("error creating directories for %s: %w", destination, err)
	}
	outFile, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", destination, err)
	}

	defer func() {
		_ = content.Close()
		_ = outFile.Close()
	}()

	_, err = io.Copy(outFile, content)
	if err != nil {
		return fmt.Errorf("error writing to file %s: %w", destination, err)
	}

	return nil
}
