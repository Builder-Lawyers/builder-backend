package build

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type TemplateBuild struct {
	storage *storage.Storage
	cfg     config.ProvisionConfig
}

func NewTemplateBuild(storage *storage.Storage, provisionConfig config.ProvisionConfig) *TemplateBuild {
	return &TemplateBuild{
		storage: storage,
		cfg:     provisionConfig,
	}
}

func (b *TemplateBuild) DownloadTemplate(ctx context.Context, templateName string) error {
	targetTemplate := filepath.Join(b.cfg.TemplatesFolder, templateName)
	exists, err := dirExists(targetTemplate)
	if err != nil {
		return err
	}
	if exists {
		dir, err := os.ReadDir(targetTemplate)
		if err != nil {
			return err
		}
		if len(dir) > 0 {
			return nil
		}
	}
	slog.Warn("Folder with templates doesn't exist, creating now")
	err = os.MkdirAll(filepath.Join(b.cfg.TemplatesFolder, templateName), fs.ModeDir)
	if err != nil {
		slog.Error("Failed to create dirs for templates", "template", err)
		return err
	}
	slog.Info("Created folders for templates")
	err = b.DownloadMissingRootFiles(ctx, b.cfg.BuildFolder, b.cfg.TemplateSrcBucketPath)
	if err != nil {
		return err
	}
	bucketPath := fmt.Sprintf("%v%v/%v", b.cfg.TemplateSrcBucketPath, "templates", templateName)
	err = b.DownloadMissingTemplateFiles(ctx, targetTemplate, bucketPath)
	if err != nil {
		return err
	}
	return nil
}

func (b *TemplateBuild) RefreshTemplate(ctx context.Context, templateName string) error {
	targetTemplate := filepath.Join(b.cfg.TemplatesFolder, templateName)
	exists, err := dirExists(targetTemplate)
	if err != nil {
		return err
	}
	if exists {
		dir, err := os.ReadDir(targetTemplate)
		if err != nil {
			return err
		}
		if len(dir) > 0 {
			return nil
		}
	}
	slog.Warn("Folder with templates doesn't exist, creating now")
	err = os.MkdirAll(filepath.Join(b.cfg.TemplatesFolder, templateName), fs.ModeDir)
	if err != nil {
		slog.Error("Failed to create dirs for templates", "template", err)
		return err
	}
	slog.Info("Created folders for templates")
	err = b.DownloadMissingRootFiles(ctx, b.cfg.BuildFolder, b.cfg.TemplateSrcBucketPath)
	if err != nil {
		return err
	}
	bucketPath := fmt.Sprintf("%v%v/%v", b.cfg.TemplateSrcBucketPath, "templates", templateName)
	err = b.DownloadTemplateFiles(ctx, targetTemplate, bucketPath)
	if err != nil {
		return err
	}
	return nil
}

func (b *TemplateBuild) DownloadMissingRootFiles(ctx context.Context, path, bucketPath string) error {
	dir, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	// 1 - because previously we MkDir all dirs to template's dir
	if len(dir) > 1 { // if only templates folder is present
		slog.Info("root dir is not empty")
		return nil
	}

	slog.Info("directory is empty, downloading sources", "path", path)
	// TODO: if there are many templates, this is bad
	files := b.storage.ListFiles(ctx, 100, &s3.ListObjectsV2Input{
		Prefix: aws.String(bucketPath),
	})
	filesToDownload := make([]string, 0)
	for _, file := range files {
		// everything under templates/
		if strings.Contains(file, "/templates/") || file == bucketPath {
			continue
		}
		filesToDownload = append(filesToDownload, file)
	}
	err = b.storage.DownloadFiles(ctx, filesToDownload, path, bucketPath)
	if err != nil {
		slog.Error("err downloading template's sources", "err", err)
		return err
	}

	return nil
}

func (b *TemplateBuild) DownloadMissingTemplateFiles(ctx context.Context, path, bucketPath string) error {
	dir, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(dir) > 0 {
		slog.Info("template's sources are present", "template", path)
		return nil
	}

	return b.DownloadTemplateFiles(ctx, path, bucketPath)
}

// Mirrors template files from s3 to local
// path - path to local dir of template
// bucketPath - path to template on S3
func (b *TemplateBuild) DownloadTemplateFiles(ctx context.Context, localPath, bucketPath string) error {

	files := b.storage.ListFiles(ctx, 100, &s3.ListObjectsV2Input{
		Prefix: aws.String(bucketPath),
	})
	err := b.storage.DownloadFiles(ctx, files, localPath, bucketPath)
	if err != nil {
		slog.Error("err downloading template's sources", "err", err)
		return err
	}

	return nil
}

// To run this, container running application has to have npm installed and "npm i -g pnpm"
func (b *TemplateBuild) RunSiteBuild(ctx context.Context, path string) (string, error) {
	templatesRootDir := filepath.Dir(path)
	if len(path) != 0 {
		slog.Info("Valid dir")
	}
	// TODO: first check if node modules exist, then run build or first install
	build := createProcess(ctx, path, "npm run build")
	err := build.Start()
	if err != nil {
		slog.Error("failed to start npm run build", "err", err)
	}

	slog.Info("npm run build started", "pid", build.Process.Pid)

	err = build.Wait()
	if err != nil {
		slog.Error("npm run build exited with err", "err", err)
		installDeps := createProcess(ctx, templatesRootDir, "pnpm i")

		err = installDeps.Start()
		if err != nil {
			slog.Error("failed to install dependencies", "err", err)
			return "", err
		}
		err = installDeps.Wait()
		if err != nil {
			slog.Error("failed to install dependencies", "err", err)
			return "", err
		}

		build = createProcess(ctx, path, "npm run build")
		err = build.Start()
		if err != nil {
			slog.Error("failed to start npm run build", "err", err)
			return "", err
		}

		slog.Info("npm run build started", "pid", build.Process.Pid)
		err = build.Wait()
		if err != nil {
			slog.Error("fatal error", "err", err)
			return "", err
		}
	}

	return path + "/dist", nil
}

// Uploads file from a filesystem path to a s3 prefix
func (b *TemplateBuild) UploadFiles(ctx context.Context, bucketPath, templateName, dir string) error {
	files := readFilesFromDir(dir)
	for _, f := range files {
		file, err := os.Open(f)
		// gets a substring after dist/
		normalized := filepath.ToSlash(f)
		parts := strings.SplitN(normalized, templateName+"/dist", 2)
		//parts := strings.SplitN(normalized, "template/"+templateName, 2)
		if err != nil {
			return fmt.Errorf("malformed filepath, %s: %v", f, err)
		}
		_, err = b.storage.UploadFile(ctx, bucketPath+parts[1], nil, file)
		if err != nil {
			return fmt.Errorf("can't put object %v", err)
		}
		if err = file.Close(); err != nil {
			return fmt.Errorf("failed to close file %s: %v", f, err)
		}
		slog.Info("Uploaded file", "fileUpload", f)
	}
	return nil
}

func (b *TemplateBuild) ClearTemplateFilesLocally(root string) error {
	exists, err := dirExists(root)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func readFilesFromDir(dir string) []string {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Error("Can't find provided directory", "dir", err)
	}
	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			subFiles := readFilesFromDir(fullPath)
			files = append(files, subFiles...)
		} else {
			files = append(files, fullPath)
		}
	}
	return files
}

func createProcess(ctx context.Context, dir string, command string) *exec.Cmd {
	log.Println(command)
	params := strings.Split(command, " ")
	proc := exec.CommandContext(ctx, params[0], params[1:]...)
	proc.Dir = dir
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	proc.Stdin = os.Stdin
	return proc
}

func dirExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}
