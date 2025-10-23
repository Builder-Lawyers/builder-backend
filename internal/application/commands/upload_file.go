package commands

import (
	"context"
	"fmt"
	"mime/multipart"

	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/storage"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/google/uuid"
)

type UploadFile struct {
	uowFactory *dbs.UOWFactory
	storage    *storage.Storage
}

func NewUploadFile(factory *dbs.UOWFactory, storage *storage.Storage) *UploadFile {
	return &UploadFile{uowFactory: factory, storage: storage}
}

func (c *UploadFile) Execute(ctx context.Context, fileHeader *multipart.FileHeader) (*dto.FileUploadedResponse, error) {
	f, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("err opening file, %v", err)
	}
	defer f.Close()

	fileID := uuid.New()
	contentType := fileHeader.Header.Get("Content-Type")
	fileURL, err := c.storage.UploadFile(ctx, fmt.Sprintf("images/%s", fileID.String()), &contentType, f)
	if err != nil {
		return nil, fmt.Errorf("err uploading to s3, %v", err)
	}

	// TODO: define what is needed to save, add files table
	// TODO: if this can be used by not registered users, set some limit to prevent ddos

	return &dto.FileUploadedResponse{
		FileID:  fileID.String(),
		FileURL: fileURL,
	}, nil
}
