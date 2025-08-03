package storage

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"log"
	"os"
	"testing"
)

var s3Client *Storage

func TestMain(m *testing.M) {
	ctx := context.Background()

	ls, err := localstack.Run(ctx,
		"localstack/localstack:1.4.0",
		testcontainers.WithEnv(map[string]string{"SERVICES": "s3"}),
	)
	if err != nil {
		log.Fatalf("failed to start localstack: %v", err)
	}
	defer func() {
		if err := ls.Terminate(ctx); err != nil {
			log.Printf("failed to terminate localstack: %s", err)
		}
	}()

	mappedPort, err := ls.MappedPort(ctx, "4566/tcp")
	if err != nil {
		log.Fatalf("failed to get port: %v", err)
	}
	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		log.Fatalf("failed to start docker provider: %v", err)
	}
	defer provider.Close()
	host, err := provider.DaemonHost(ctx)
	if err != nil {
		log.Fatalf("failed to get hostt: %v", err)
	}

	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("AWS_ENDPOINT_URL", "http://"+host+":"+mappedPort.Port())

	s3Client = NewStorage()

	exitCode := m.Run()

	if err := ls.Terminate(ctx); err != nil {
		log.Printf("failed to terminate localstack: %s", err)
	}

	os.Exit(exitCode)
}

func TestListFilesEmpty(t *testing.T) {
	bucket := "sanity-web"
	_, err := s3Client.client.CreateBucket(context.Background(), &s3.CreateBucketInput{Bucket: &bucket})
	if err != nil {
		log.Fatalf("Failed to create bucket %v", err)
	}
	files := s3Client.ListFiles(1, "")
	assert.Empty(t, files, "files found should be empty")

}
