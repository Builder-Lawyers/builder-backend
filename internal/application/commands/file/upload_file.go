package file

import (
	"context"
	"fmt"
	"mime/multipart"

	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/storage"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/pkg/env"
	"github.com/google/uuid"
)

type UploadFile struct {
	uowFactory *dbs.UOWFactory
	storage    *storage.Storage
	cfg        UploadConfig
}

func NewUploadFile(factory *dbs.UOWFactory, storage *storage.Storage, cfg UploadConfig) *UploadFile {
	return &UploadFile{uowFactory: factory, storage: storage, cfg: cfg}
}

type UploadConfig struct {
	path string
}

func NewUploadConfig() UploadConfig {
	return UploadConfig{
		path: env.GetEnv("UPLOAD_PREFIX", "images/"),
	}
}

func (c *UploadFile) Execute(ctx context.Context, fileHeader *multipart.FileHeader) (*dto.FileUploadedResponse, error) {
	f, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("err opening file, %v", err)
	}
	defer f.Close()

	fileID := uuid.New()
	contentType := fileHeader.Header.Get("Content-Type")
	fileURL, err := c.storage.UploadFile(ctx, fmt.Sprintf("%s%s", c.cfg.path, fileID.String()), &contentType, f)
	if err != nil {
		return nil, fmt.Errorf("err uploading to s3, %v", err)
	}

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}
	defer uow.Finalize(&err)

	_, err = tx.Exec(ctx, "INSERT INTO builder.files(id) VALUES($1)", fileID)
	if err != nil {
		return nil, fmt.Errorf("err inserting file to db %v", err)
	}

	// TODO: define what is needed to save, add files table
	// TODO: if this can be used by not registered users, set some limit to prevent ddos

	return &dto.FileUploadedResponse{
		FileID:  fileID.String(),
		FileURL: fileURL,
	}, nil
}
