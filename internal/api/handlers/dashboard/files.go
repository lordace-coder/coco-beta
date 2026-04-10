package dashboard

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/pkg/config"
)

// ListFiles handles GET /_/api/projects/:id/files
// Lists files stored in B2/S3 under the project's prefix.
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
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Storage not configured"})
	}

	out, err := client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to list files"})
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

	return c.JSON(fiber.Map{
		"data":  files,
		"total": len(files),
	})
}

// DeleteFile handles DELETE /_/api/projects/:id/files
// Body: {"key": "projectID/filename.ext"}
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

	// Ensure the key belongs to this project
	if !strings.HasPrefix(req.Key, projectID+"/") {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": true, "message": "Key does not belong to this project"})
	}

	client, bucket, err := newS3Client()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Storage not configured"})
	}

	_, err = client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(req.Key),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to delete file"})
	}

	return c.JSON(fiber.Map{"message": "File deleted", "key": req.Key})
}

func newS3Client() (*s3.S3, string, error) {
	cfg := config.AppConfig
	if cfg == nil || cfg.BucketEndpoint == "" || cfg.BackblazeKeyID == "" {
		return nil, "", fiber.ErrInternalServerError
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
	// Use the standard Backblaze public URL format
	return "https://f005.backblazeb2.com/file/" + cfg.BucketName + "/" + key
}
