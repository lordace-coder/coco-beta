package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gofiber/fiber/v2"
	applogger "github.com/patrick/cocobase/pkg/logger"
	"github.com/patrick/cocobase/pkg/config"
)

// ListFiles handles GET /_/api/projects/:id/files
func ListFiles(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	prefix := c.Query("prefix", "")
	if prefix == "" {
		prefix = projectID + "/"
	} else if !strings.HasPrefix(prefix, projectID+"/") {
		prefix = projectID + "/" + prefix
	}

	client, bucket, err := newS3Client()
	if err != nil {
		applogger.Error("ListFiles: storage not configured or S3 init failed: %v", err)
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error":   true,
			"message": "Storage is not configured. Add BACKBLAZE_KEY_ID, BACKBLAZE_APPLICATION_KEY, BUCKET_NAME, and BUCKET_ENDPOINT to your .env",
		})
	}

	out, err := client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		applogger.Error("ListFiles: S3 ListObjectsV2 failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": fmt.Sprintf("Failed to list files: %v", err)})
	}

	type fileEntry struct {
		Key          string    `json:"key"`
		Size         int64     `json:"size"`
		LastModified time.Time `json:"last_modified"`
		URL          string    `json:"url"`
	}

	files := make([]fileEntry, 0, len(out.Contents))
	cfg := config.AppConfig
	for _, obj := range out.Contents {
		key := aws.StringValue(obj.Key)
		files = append(files, fileEntry{
			Key:          key,
			Size:         aws.Int64Value(obj.Size),
			LastModified: aws.TimeValue(obj.LastModified),
			URL:          buildFileURL(cfg, key),
		})
	}

	return c.JSON(fiber.Map{"data": files, "total": len(files)})
}

// DeleteFile handles DELETE /_/api/projects/:id/files
func DeleteFile(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	var req struct {
		Key string `json:"key"`
	}
	if err := c.BodyParser(&req); err != nil || req.Key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "key is required"})
	}

	if !strings.HasPrefix(req.Key, projectID+"/") {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": true, "message": "Key does not belong to this project"})
	}

	client, bucket, err := newS3Client()
	if err != nil {
		applogger.Error("DeleteFile: storage not configured or S3 init failed: %v", err)
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error":   true,
			"message": "Storage is not configured. Add BACKBLAZE_KEY_ID, BACKBLAZE_APPLICATION_KEY, BUCKET_NAME, and BUCKET_ENDPOINT to your .env",
		})
	}

	_, err = client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(req.Key),
	})
	if err != nil {
		applogger.Error("DeleteFile: S3 DeleteObject failed for key %s: %v", req.Key, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": fmt.Sprintf("Failed to delete file: %v", err)})
	}

	return c.JSON(fiber.Map{"message": "File deleted", "key": req.Key})
}

func newS3Client() (*s3.S3, string, error) {
	cfg := config.AppConfig
	if cfg == nil || cfg.BucketEndpoint == "" || cfg.BackblazeKeyID == "" {
		return nil, "", fmt.Errorf("storage environment variables not set (BACKBLAZE_KEY_ID, BUCKET_ENDPOINT)")
	}

	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("us-east-005"),
		Endpoint:         aws.String(cfg.BucketEndpoint),
		Credentials:      credentials.NewStaticCredentials(cfg.BackblazeKeyID, cfg.BackblazeApplicationKey, ""),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		return nil, "", err
	}
	return s3.New(sess), cfg.BucketName, nil
}

func buildFileURL(cfg *config.Config, key string) string {
	if cfg.BucketName == "" {
		return ""
	}
	return "https://f005.backblazeb2.com/file/" + cfg.BucketName + "/" + key
}
