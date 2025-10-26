package file_test

import (
	"bytes"
	"context"
	"io"
	"log"
	"log/slog"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Builder-Lawyers/builder-backend/internal/application/commands/file"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/storage"
	"github.com/Builder-Lawyers/builder-backend/internal/testinfra"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/pkg/env"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var s3Storage *storage.Storage
var s3Client *s3.Client
var uowFactory = db.NewUoWFactory(testinfra.Pool)
var bucketName = "sanity-web"
var prefix = env.GetEnv("UPLOAD_PREFIX", "test-images/")

func TestMain(m *testing.M) {
	ctx := context.Background()
	s3Client = s3.NewFromConfig(testinfra.AwsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	s3Storage = storage.NewStorage(testinfra.AwsCfg)
	exitCode := m.Run()

	slog.Info("Finished uploadFile tests")

	cleanup(ctx)

	os.Exit(exitCode)
}

func Test_Upload_File_When_Called_With_Valid_File_Then_Inserted_In_DB_And_Uploaded_To_S3(t *testing.T) {
	ctx := context.Background()
	uploadConfig := file.NewUploadConfig()
	SUT := file.NewUploadFile(uowFactory, s3Storage, uploadConfig)

	resp, err := SUT.Execute(ctx, getRequestFileHeader())
	require.NoError(t, err)
	require.NotEmpty(t, resp.FileURL)

	var createdFileID string
	err = testinfra.Pool.QueryRow(ctx, "SELECT id FROM builder.files WHERE id = $1", uuid.MustParse(resp.FileID)).Scan(&createdFileID)
	require.NoError(t, err)

	require.Equal(t, resp.FileID, createdFileID)
	files := s3Storage.ListFiles(ctx, 1, &s3.ListObjectsV2Input{Bucket: aws.String(bucketName), Prefix: aws.String(prefix)})
	require.Len(t, files, 1)
	objectKey := files[0][strings.LastIndex(files[0], "/")+1:] // get object key after prefix
	require.Equal(t, createdFileID, objectKey)
}

func getRequestFileHeader() *multipart.FileHeader {
	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)

	f, err := os.Open(filepath.Join("testdata", "test-file.jpg"))
	if err != nil {
		log.Panic(err)
	}
	defer f.Close()

	fw, err := w.CreateFormFile("file", "test-file.jpg")
	if err != nil {
		log.Panic(err)
	}
	if _, err := io.Copy(fw, f); err != nil {
		log.Panic(err)
	}

	w.Close()

	req := httptest.NewRequest("POST", "/", buf)
	req.Header.Set("Content-Type", w.FormDataContentType())

	_, mp, err := req.FormFile("file")
	if err != nil {
		log.Panic(err)
	}
	return mp
}

func cleanup(ctx context.Context) {
	out, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		slog.Error("err cleaning up s3 after tests", "err", err)
	}

	if len(out.Contents) == 0 {
		slog.Info("s3 prefix already cleaned up")
		return
	}

	objsToDelete := make([]types.ObjectIdentifier, 0, len(out.Contents))
	for _, o := range out.Contents {
		objsToDelete = append(objsToDelete, types.ObjectIdentifier{Key: o.Key})
	}

	slog.Info("cleaning up test files from s3", "files", len(objsToDelete))

	_, err = s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: &types.Delete{Objects: objsToDelete, Quiet: aws.Bool(true)},
	})
}
