package storage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Builder-Lawyers/builder-backend/pkg/env"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Storage struct {
	client *s3.Client
	bucket string
}

func NewStorage(config aws.Config) *Storage {
	return &Storage{
		initClient(config),
		env.GetEnv("S3_BUCKET", "sanity-web"),
	}
}

func initClient(config aws.Config) *s3.Client {
	client := s3.NewFromConfig(config, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	return client
}

func (s *Storage) UploadFile(key string, contentType *string, body io.Reader) error {
	var ct string

	data, err := io.ReadAll(body)
	if err != nil && err != io.EOF {
		return fmt.Errorf("reading for content-type detection: %w", err)
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
	_, err = s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentType:   aws.String(ct),
		ContentLength: aws.Int64(int64(len(data))),
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *Storage) ListFiles(limit int32, prefix string) []string {
	params := &s3.ListObjectsV2Input{
		Bucket: &s.bucket,
		Prefix: &prefix,
	}
	p := s3.NewListObjectsV2Paginator(s.client, params, func(o *s3.ListObjectsV2PaginatorOptions) {
		o.Limit = limit
	})

	var i int
	var files []string
	for p.HasMorePages() {
		i++
		page, err := p.NextPage(context.TODO())
		if err != nil {
			log.Fatalf("failed to get page %v, %v", i, err)
		}
		for _, obj := range page.Contents {
			files = append(files, *obj.Key)
		}
	}
	return files
}

func (s *Storage) DownloadFiles(keys []string, destination string) error {
	bucket := "sanity-web"
	for _, key := range keys {
		params := &s3.GetObjectInput{
			Bucket: &bucket,
			Key:    aws.String(key),
		}
		resp, err := s.client.GetObject(context.Background(), params)
		if err != nil {
			return fmt.Errorf("error downloading key %s: %w", key, err)
		}

		return s.readAndCopyObjectTo(resp.Body, filepath.Join(destination, key))
	}
	return nil
}

func (s *Storage) readAndCopyObjectTo(content io.ReadCloser, destination string) error {
	slog.Info("saving file to %v", destination)
	defer content.Close()
	if err := os.MkdirAll(filepath.Dir(destination), os.ModePerm); err != nil {
		return fmt.Errorf("error creating directories for %s: %w", destination, err)
	}
	outFile, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", destination, err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, content)
	if err != nil {
		return fmt.Errorf("error writing to file %s: %w", destination, err)
	}

	return nil
}
