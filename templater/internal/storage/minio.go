package storage

import (
	"builder-templater/pkg/env"
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"log"
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
		env.GetEnv("S3_BUCKET", "test-web"),
	}
}

func initClient() *s3.Client {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Println(err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	return client
}

func (s Storage) UploadFile(key, contentType string, body []byte) error {
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
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
			fmt.Println("Object:", *obj.Key)
			files = append(files, *obj.Key)
		}
	}
	return files
}

func (s Storage) UploadFiles(dir string) {
	var files []string
	files = readFilesFromDir(dir, files)
	for _, f := range files {
		file, err := os.Open(f)
		if err != nil {
			log.Printf("Can't open file %s: %v", f, err)
			continue
		}
		params := s3.PutObjectInput{
			Bucket: aws.String("sanity-web"),
			Key:    aws.String(f),
			Body:   file,
		}
		_, err = s.client.PutObject(context.Background(), &params)
		if err != nil {
			fmt.Printf("Can't put object %v", err)
		}
		log.Printf("Uploaded file %v", f)
		if err := file.Close(); err != nil {
			log.Printf("Failed to close file %s: %v", f, err)
		}
	}
}

func readFilesFromDir(dir string, files []string) []string {
	directory, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("Can't find provided directory %v", err)
	}
	for _, v := range directory {
		fileName := filepath.Join(dir, v.Name())
		if v.IsDir() {
			readFilesFromDir(fileName, files)
		} else {
			files = append(files, fileName)
		}
	}
	return files
}
