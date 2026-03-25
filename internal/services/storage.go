package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/pkg/config"
)

var (
	s3Client      *s3.S3
	bucketName    string
	publicBaseURL string
)

// InitializeS3 initializes the S3 client for Backblaze B2
func InitializeS3() error {
	cfg := config.AppConfig
	if cfg == nil {
		return fmt.Errorf("config not initialized")
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

// GetProjectStorageUsage gets current storage usage for a project in bytes
func GetProjectStorageUsage(projectID string) (int64, error) {
	if s3Client == nil {
		return 0, fmt.Errorf("S3 client not initialized")
	}

	prefix := fmt.Sprintf("projects/%s/", projectID)
	var totalSize int64

	err := s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			totalSize += *obj.Size
		}
		return true // Continue to next page
	})

	if err != nil {
		log.Printf("Failed to get storage usage for project %s: %v", projectID, err)
		return 0, err
	}

	return totalSize, nil
}

// CheckStorageLimit checks if uploading a file would exceed storage limit
func CheckStorageLimit(projectID string, fileSize int64) error {
	// Get project's current plan
	var project models.Project
	if err := database.DB.Where("id = ?", projectID).First(&project).Error; err != nil {
		return fmt.Errorf("project not found: %w", err)
	}

	plan := GetCurrentPlan(&project)

	// Check if storage is unlimited (nil or 0)
	if plan.MaxStorageMB == nil || *plan.MaxStorageMB == 0 {
		return nil // Unlimited storage
	}

	storageLimitBytes := int64(*plan.MaxStorageMB) * 1024 * 1024

	// Get current usage
	currentUsage, err := GetProjectStorageUsage(projectID)
	if err != nil {
		return fmt.Errorf("failed to check storage usage: %w", err)
	}

	// Check if adding this file would exceed limit
	if currentUsage+fileSize > storageLimitBytes {
		// Send notification in background
		go func() {
			// TODO: Implement notification
			log.Printf("⚠️ Storage limit reached for project %s: %d/%d MB",
				projectID, currentUsage/1024/1024, plan.MaxStorageMB)
		}()

		return fmt.Errorf(
			"storage limit exceeded. Current: %.2f MB, Limit: %d MB, File: %.2f MB",
			float64(currentUsage)/1024/1024,
			plan.MaxStorageMB,
			float64(fileSize)/1024/1024,
		)
	}

	return nil
}

// FileUploadResult contains the result of a file upload
type FileUploadResult struct {
	Filename     string    `json:"filename"`
	URL          string    `json:"url"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag"`
}

// UploadFile uploads a file to S3 and returns the public URL
func UploadFile(ctx context.Context, fileContent []byte, projectID, filename, subdirectory string) (*FileUploadResult, error) {
	if s3Client == nil {
		return nil, fmt.Errorf("S3 client not initialized")
	}

	// Check storage limit before upload
	if err := CheckStorageLimit(projectID, int64(len(fileContent))); err != nil {
		return nil, err
	}

	// Construct S3 key
	key := fmt.Sprintf("projects/%s", projectID)
	if subdirectory != "" {
		key = fmt.Sprintf("%s/%s", key, subdirectory)
	}
	key = fmt.Sprintf("%s/%s", key, filename)

	// Upload to S3
	_, err := s3Client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader(fileContent),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	log.Printf("✅ Successfully uploaded '%s' to S3 at '%s'", filename, key)

	return &FileUploadResult{
		Filename:     filename,
		URL:          publicBaseURL + key,
		Size:         int64(len(fileContent)),
		LastModified: time.Now(),
	}, nil
}

// GetFiles lists all files for a project
func GetFiles(projectID, subdirectory string) ([]*FileUploadResult, error) {
	if s3Client == nil {
		return nil, fmt.Errorf("S3 client not initialized")
	}

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
			filename := key[len(prefix):]

			// Skip empty directory placeholders
			if filename == "" || len(filename) == 0 {
				continue
			}

			files = append(files, &FileUploadResult{
				Filename:     filename,
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

// DeleteFile deletes a file from S3
func DeleteFile(projectID, filename, subdirectory string) error {
	if s3Client == nil {
		return fmt.Errorf("S3 client not initialized")
	}

	key := fmt.Sprintf("projects/%s", projectID)
	if subdirectory != "" {
		key = fmt.Sprintf("%s/%s", key, subdirectory)
	}
	key = fmt.Sprintf("%s/%s", key, filename)

	_, err := s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	log.Printf("✅ Successfully deleted '%s' from S3", key)
	return nil
}

// DownloadFile downloads a file from S3
func DownloadFile(projectID, filename, subdirectory string) ([]byte, error) {
	if s3Client == nil {
		return nil, fmt.Errorf("S3 client not initialized")
	}

	key := fmt.Sprintf("projects/%s", projectID)
	if subdirectory != "" {
		key = fmt.Sprintf("%s/%s", key, subdirectory)
	}
	key = fmt.Sprintf("%s/%s", key, filename)

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
