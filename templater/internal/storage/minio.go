package storage

import (
	"context"
	"fmt"
	"github.com/Builder-Lawyers/builder-backend/pkg/env"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
)

type Storage struct {
	client *s3.Client
	bucket string
}

func NewStorage() *Storage {
	return &Storage{
		initClient(),
		env.GetEnv("S3_BUCKET", "sanity-web"),
	}
}

func initClient() *s3.Client {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		slog.Info("", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	return client
}

func (s Storage) UploadFile(key string, contentType *string, body io.Reader) error {
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: contentType,
	})
	if err != nil {
		return err
	}
	return nil
}

func (s Storage) ListFiles() []string {
	bucket := "sanity-web"
	params := &s3.ListObjectsV2Input{
		Bucket: &bucket,
	}
	p := s3.NewListObjectsV2Paginator(s.client, params, func(o *s3.ListObjectsV2PaginatorOptions) {
		if v := int32(3); v != 0 {
			o.Limit = v
		}
	})

	var i int
	log.Println("Objects:")
	var files []string
	for p.HasMorePages() {
		i++

		// Next Page takes a new context for each page retrieval. This is where
		// you could add timeouts or deadlines.
		page, err := p.NextPage(context.TODO())
		if err != nil {
			log.Fatalf("failed to get page %v, %v", i, err)
		}

		// Log the objects found
		for _, obj := range page.Contents {
			slog.Info("Object:", *obj.Key)
			files = append(files, *obj.Key)
		}
	}
	return files
}

func (s Storage) DownloadFiles(keys []string, destination string) error {
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

func (s Storage) readAndCopyObjectTo(content io.ReadCloser, destination string) error {
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
