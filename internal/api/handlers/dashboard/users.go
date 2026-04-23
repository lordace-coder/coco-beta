package dashboard

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/instance"
	"github.com/google/uuid"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// CreateUser handles POST /_/api/projects/:id/users
func CreateUser(c *fiber.Ctx) error {
	projectID := instance.ID()
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	var req struct {
		Email    string                 `json:"email"`
		Password string                 `json:"password"`
		Data     map[string]interface{} `json:"data"`
		Roles    models.StringArray     `json:"roles"`
	}
	if err := c.BodyParser(&req); err != nil || req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "email and password are required"})
	}

	// Check duplicate
	var count int64
	database.DB.Model(&models.AppUser{}).Where("email = ? AND client_id = ?", strings.ToLower(req.Email), projectID).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": true, "message": "User with this email already exists"})
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to hash password"})
	}

	data := models.JSONMap{}
	if req.Data != nil {
		data = models.JSONMap(req.Data)
	}
	roles := req.Roles
	if roles == nil {
		roles = models.StringArray{}
	}

	user := models.AppUser{
		ID:       uuid.New().String(),
		ClientID: projectID,
		Email:    strings.ToLower(req.Email),
		Password: string(hashed),
		Data:     data,
		Roles:    roles,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to create user"})
	}

	Log(projectID, "create_user", "user", user.ID, user.Email)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":    user.ID,
		"email": user.Email,
		"data":  user.Data,
		"roles": user.Roles,
	})
}

// ListUsers handles GET /_/api/projects/:id/users
func ListUsers(c *fiber.Ctx) error {
	projectID := instance.ID()
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	if limit > 500 {
		limit = 500
	}

	var total int64
	database.DB.Model(&models.AppUser{}).Where("client_id = ?", projectID).Count(&total)

	var users []models.AppUser
	if err := database.DB.Where("client_id = ?", projectID).
		Order("created_at desc").Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to fetch users"})
	}

	// Sanitise — never return passwords
	type safeUser struct {
		ID            string                 `json:"id"`
		Email         string                 `json:"email"`
		Data          map[string]interface{} `json:"data"`
		Roles         []string               `json:"roles"`
		EmailVerified bool                   `json:"email_verified"`
		OAuthProvider *string                `json:"oauth_provider,omitempty"`
		CreatedAt     string                 `json:"created_at"`
	}

	safe := make([]safeUser, len(users))
	for i, u := range users {
		safe[i] = safeUser{
			ID:            u.ID,
			Email:         u.Email,
			Data:          u.Data,
			Roles:         u.Roles,
			EmailVerified: u.EmailVerified,
			OAuthProvider: u.OAuthProvider,
			CreatedAt:     u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return c.JSON(fiber.Map{
		"data":     safe,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"has_more": int64(offset+limit) < total,
	})
}

// GetUser handles GET /_/api/projects/:id/users/:userId
func GetUser(c *fiber.Ctx) error {
	var user models.AppUser
	if err := database.DB.Where("id = ? AND client_id = ?", c.Params("userId"), instance.ID()).First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "User not found"})
	}
	return c.JSON(fiber.Map{
		"id":             user.ID,
		"email":          user.Email,
		"data":           user.Data,
		"roles":          user.Roles,
		"email_verified": user.EmailVerified,
		"oauth_provider": user.OAuthProvider,
		"created_at":     user.CreatedAt,
	})
}

// UpdateUser handles PATCH /_/api/projects/:id/users/:userId
func UpdateUser(c *fiber.Ctx) error {
	var user models.AppUser
	if err := database.DB.Where("id = ? AND client_id = ?", c.Params("userId"), instance.ID()).First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "User not found"})
	}

	var req struct {
		Data          *map[string]interface{} `json:"data"`
		Roles         models.StringArray      `json:"roles"`
		EmailVerified *bool                   `json:"email_verified"`
		Override      bool                    `json:"override"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}

	updates := map[string]interface{}{}

	if req.Data != nil {
		if req.Override {
			user.Data = models.JSONMap(*req.Data)
		} else {
			for k, v := range *req.Data {
				user.Data[k] = v
			}
		}
		updates["data"] = user.Data
	}
	if req.Roles != nil {
		updates["roles"] = req.Roles
	}
	if req.EmailVerified != nil {
		updates["email_verified"] = *req.EmailVerified
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&user).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to update user"})
		}
	}

	return c.JSON(fiber.Map{"message": "User updated", "id": user.ID})
}

// DeleteUser handles DELETE /_/api/projects/:id/users/:userId
func DeleteUser(c *fiber.Ctx) error {
	var user models.AppUser
	if err := database.DB.Where("id = ? AND client_id = ?", c.Params("userId"), instance.ID()).First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "User not found"})
	}
	database.DB.Delete(&user)
	Log(instance.ID(), "delete_user", "user", user.ID, user.Email)
	return c.JSON(fiber.Map{"message": "User deleted"})
}

// DeleteAllUsers handles DELETE /_/api/projects/:id/users
func DeleteAllUsers(c *fiber.Ctx) error {
	projectID := instance.ID()
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}
	var result = database.DB.Where("client_id = ?", projectID).Delete(&models.AppUser{})
	return c.JSON(fiber.Map{"message": "All users deleted", "deleted": result.RowsAffected})
}
