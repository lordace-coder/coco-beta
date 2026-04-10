package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/patrick/cocobase/pkg/config"
)

var (
	s3Client      *s3.S3
	bucketName    string
	publicBaseURL string

	// localUploadsDir is the root directory for local file storage (fallback when S3 is not configured).
	localUploadsDir = "uploads"
)

// InitializeS3 initializes the S3 client for Backblaze B2.
// If the required env vars are not set it logs a notice and leaves s3Client nil,
// which causes all storage operations to fall back to local disk.
func InitializeS3() error {
	cfg := config.AppConfig
	if cfg == nil {
		return fmt.Errorf("config not initialized")
	}

	// If none of the S3/Backblaze vars are set, silently skip S3 and use local storage.
	if cfg.BackblazeKeyID == "" && cfg.BucketName == "" && cfg.BucketEndpoint == "" {
		log.Println("ℹ️  No S3/Backblaze config found — using local disk storage (uploads/)")
		return nil
	}

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-005"),
		Endpoint:    aws.String(cfg.BucketEndpoint),
		Credentials: credentials.NewStaticCredentials(cfg.BackblazeKeyID, cfg.BackblazeApplicationKey, ""),
	})

	if err != nil {
		return fmt.Errorf("failed to create S3 session: %w", err)
	}

	s3Client = s3.New(sess)
	bucketName = cfg.BucketName
	publicBaseURL = fmt.Sprintf("https://f005.backblazeb2.com/file/%s/", bucketName)

	log.Printf("✅ S3 client initialized for bucket: %s", bucketName)
	return nil
}

// IsLocalStorage reports whether files are stored on local disk (i.e. S3 is not configured).
func IsLocalStorage() bool {
	return s3Client == nil
}

// LocalUploadsDir returns the root directory used for local file storage.
func LocalUploadsDir() string {
	return localUploadsDir
}

// FileUploadResult contains the result of a file upload
type FileUploadResult struct {
	Filename     string    `json:"filename"`
	URL          string    `json:"url"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag"`
}

// UploadFile uploads a file. When S3 is configured it uploads to the bucket;
// otherwise it saves to the local uploads/ directory and returns a relative URL.
func UploadFile(ctx context.Context, fileContent []byte, projectID, filename, subdirectory string) (*FileUploadResult, error) {
	if s3Client != nil {
		return uploadToS3(ctx, fileContent, projectID, filename, subdirectory)
	}
	return uploadToLocal(fileContent, projectID, filename, subdirectory)
}

func uploadToS3(ctx context.Context, fileContent []byte, projectID, filename, subdirectory string) (*FileUploadResult, error) {
	key := buildKey(projectID, subdirectory, filename)

	_, err := s3Client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader(fileContent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	log.Printf("✅ Uploaded '%s' to S3 at '%s'", filename, key)

	return &FileUploadResult{
		Filename:     filename,
		URL:          publicBaseURL + key,
		Size:         int64(len(fileContent)),
		LastModified: time.Now(),
	}, nil
}

func uploadToLocal(fileContent []byte, projectID, filename, subdirectory string) (*FileUploadResult, error) {
	rel := buildKey(projectID, subdirectory, filename) // projects/<id>/<subdir>/<file>
	dst := filepath.Join(localUploadsDir, filepath.FromSlash(rel))

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}
	if err := os.WriteFile(dst, fileContent, 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	log.Printf("✅ Saved file locally at '%s'", dst)

	// URL is the public path that Fiber's static middleware will serve.
	url := "/uploads/" + rel

	return &FileUploadResult{
		Filename:     filename,
		URL:          url,
		Size:         int64(len(fileContent)),
		LastModified: time.Now(),
	}, nil
}

// GetProjectStorageUsage gets current storage usage for a project in bytes.
func GetProjectStorageUsage(projectID string) (int64, error) {
	if s3Client != nil {
		return s3StorageUsage(projectID)
	}
	return localStorageUsage(projectID)
}

func s3StorageUsage(projectID string) (int64, error) {
	prefix := fmt.Sprintf("projects/%s/", projectID)
	var totalSize int64

	err := s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			totalSize += *obj.Size
		}
		return true
	})
	if err != nil {
		log.Printf("Failed to get storage usage for project %s: %v", projectID, err)
		return 0, err
	}
	return totalSize, nil
}

func localStorageUsage(projectID string) (int64, error) {
	root := filepath.Join(localUploadsDir, "projects", projectID)
	var totalSize int64
	err := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		totalSize += info.Size()
		return nil
	})
	return totalSize, err
}

// GetFiles lists all files for a project.
func GetFiles(projectID, subdirectory string) ([]*FileUploadResult, error) {
	if s3Client != nil {
		return s3GetFiles(projectID, subdirectory)
	}
	return localGetFiles(projectID, subdirectory)
}

func s3GetFiles(projectID, subdirectory string) ([]*FileUploadResult, error) {
	prefix := fmt.Sprintf("projects/%s/", projectID)
	if subdirectory != "" {
		prefix = fmt.Sprintf("%s%s/", prefix, subdirectory)
	}

	var files []*FileUploadResult

	err := s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			key := *obj.Key
			name := key[len(prefix):]
			if name == "" {
				continue
			}
			files = append(files, &FileUploadResult{
				Filename:     name,
				URL:          publicBaseURL + key,
				Size:         *obj.Size,
				LastModified: *obj.LastModified,
				ETag:         *obj.ETag,
			})
		}
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	return files, nil
}

func localGetFiles(projectID, subdirectory string) ([]*FileUploadResult, error) {
	root := filepath.Join(localUploadsDir, "projects", projectID)
	if subdirectory != "" {
		root = filepath.Join(root, subdirectory)
	}

	var files []*FileUploadResult

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		rel, _ := filepath.Rel(filepath.Join(localUploadsDir), path)
		url := "/uploads/" + filepath.ToSlash(rel)

		files = append(files, &FileUploadResult{
			Filename:     info.Name(),
			URL:          url,
			Size:         info.Size(),
			LastModified: info.ModTime(),
		})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return files, nil
}

// DeleteFile deletes a file from S3 or local disk.
func DeleteFile(projectID, filename, subdirectory string) error {
	if s3Client != nil {
		return s3DeleteFile(projectID, filename, subdirectory)
	}
	return localDeleteFile(projectID, filename, subdirectory)
}

func s3DeleteFile(projectID, filename, subdirectory string) error {
	key := buildKey(projectID, subdirectory, filename)

	_, err := s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	log.Printf("✅ Deleted '%s' from S3", key)
	return nil
}

func localDeleteFile(projectID, filename, subdirectory string) error {
	rel := buildKey(projectID, subdirectory, filename)
	dst := filepath.Join(localUploadsDir, filepath.FromSlash(rel))
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	log.Printf("✅ Deleted local file '%s'", dst)
	return nil
}

// DownloadFile downloads a file from S3 or reads it from local disk.
func DownloadFile(projectID, filename, subdirectory string) ([]byte, error) {
	if s3Client != nil {
		return s3DownloadFile(projectID, filename, subdirectory)
	}
	return localDownloadFile(projectID, filename, subdirectory)
}

func s3DownloadFile(projectID, filename, subdirectory string) ([]byte, error) {
	key := buildKey(projectID, subdirectory, filename)

	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}
	return data, nil
}

func localDownloadFile(projectID, filename, subdirectory string) ([]byte, error) {
	rel := buildKey(projectID, subdirectory, filename)
	dst := filepath.Join(localUploadsDir, filepath.FromSlash(rel))
	data, err := os.ReadFile(dst)
	if err != nil {
		return nil, fmt.Errorf("failed to read local file: %w", err)
	}
	return data, nil
}

// buildKey returns the canonical path/key: projects/<projectID>[/<subdir>]/<filename>
func buildKey(projectID, subdirectory, filename string) string {
	key := fmt.Sprintf("projects/%s", projectID)
	if subdirectory != "" {
		key = fmt.Sprintf("%s/%s", key, subdirectory)
	}
	return fmt.Sprintf("%s/%s", key, filename)
}
