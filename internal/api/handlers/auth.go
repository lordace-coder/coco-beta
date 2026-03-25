package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/dto"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
	"gorm.io/gorm"
)

// Google OAuth constants
const (
	// Google OAuth Integration ID (same as Python)
	GoogleOAuthIntegrationID = "046deb41-47b3-403d-aee8-b80ccb80a87e"

	// Google OAuth endpoints
	GoogleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL    = "https://oauth2.googleapis.com/token"
	GoogleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
)

// GoogleOAuthConfig holds OAuth configuration
type GoogleOAuthConfig struct {
	ClientID     string `json:"GOOGLE_CLIENT_ID"`
	ClientSecret string `json:"GOOGLE_CLIENT_SECRET"`
	RedirectURL  string `json:"GOOGLE_REDIRECT_URL"`
	CompleteURL  string `json:"GOOGLE_COMPLETE_URL"`
}

// GoogleUserInfo represents user data from Google
type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// GoogleTokenResponse represents OAuth token response
type GoogleTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// UserLogin authenticates an app user and returns a JWT token
// @Summary App user login
// @Description Authenticate an app user with email and password
// @Tags App Client
// @Accept json
// @Produce json
// @Param credentials body dto.AppUserLoginRequest true "Login credentials"
// @Success 200 {object} dto.TokenResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /auth-collections/login [post]
func UserLogin(c *fiber.Ctx) error {
	// TODO - ADD PROJECT CACHE LAYER TO AVOID DB LOOKUP ON EVERY LOGIN REQUEST, ASS WELL AS CACHING OF THE PROJECTS PLAN
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	var req dto.AppUserLoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	// Find user
	var user models.AppUser
	if err := database.DB.Where("client_id = ? AND email = ?", project.ID, req.Email).First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Account with this email does not exist",
		})
	}

	// Check password
	if !user.ComparePassword(req.Password) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid password value",
		})
	}

	// TODO Check for 2fa first here

	// Generate token
	token, err := services.CreateAppUserToken(&user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to generate token",
		})
	}

	userDto := dto.AppUserToResponse(&user)
	return c.JSON(dto.TokenResponse{AccessToken: token, User: userDto})
}

// UserSignup creates a new app user and returns a JWT token
// @Summary App user signup
// @Description Create a new app user account
// @Tags App Client
// @Accept json
// @Produce json
// @Param user body dto.AppUserSignupRequest true "User data"
// @Success 200 {object} dto.TokenResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /auth-collections/signup [post]
func UserSignup(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	var req dto.AppUserSignupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	// Check if user already exists
	var existingUser models.AppUser
	if err := database.DB.Where("client_id = ? AND email = ?", project.ID, req.Email).First(&existingUser).Error; err == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "User with this email already exists",
		})
	}

	// TODO: Check user limits based on pricing plan
	// This would involve querying the pricing table and checking max_users

	// Initialize Data map if nil
	if req.Data == nil {
		req.Data = make(map[string]interface{})
	}

	// Create new user
	user := models.AppUser{
		ClientID: project.ID,
		Email:    req.Email,
		Data:     req.Data,
		Roles:    models.StringArray{}, // Initialize empty roles array
	}

	if err := user.SetPassword(req.Password); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to hash password",
		})
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to create user",
		})
	}

	// Generate token
	token, err := services.CreateAppUserToken(&user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to generate token",
		})
	}

	return c.JSON(dto.TokenResponse{AccessToken: token, User: dto.AppUserToResponse(&user)})
}

// ListAllUsers returns all users for a project
// @Summary List app users
// @Description Get all app users for the current project
// @Tags App Client
// @Produce json
// @Success 200 {array} dto.AppUserResponse
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /auth-collections/users [get]
func ListAllUsers(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	var users []models.AppUser
	if err := database.DB.Where("client_id = ?", project.ID).Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to fetch users",
		})
	}

	// Convert to response format
	response := make([]dto.AppUserResponse, len(users))
	for i, user := range users {
		response[i] = dto.AppUserToResponse(&user)
	}

	return c.JSON(response)
}

// GetUserByID returns a specific user by ID
// @Summary Get app user by ID
// @Description Get a specific app user by their ID
// @Tags App Client
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} dto.AppUserResponse
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /auth-collections/users/{id} [get]
func GetUserByID(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	userID := c.Params("id")
	var user models.AppUser
	if err := database.DB.Where("client_id = ? AND id = ?", project.ID, userID).First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "User not found",
		})
	}

	return c.JSON(dto.AppUserToResponse(&user))
}

// GetCurrentUser returns the currently authenticated user
// @Summary Get current app user
// @Description Get the currently authenticated app user's details
// @Tags App Client
// @Produce json
// @Success 200 {object} dto.AppUserResponse
// @Failure 401 {object} map[string]interface{}
// @Security BearerAuth
// @Router /auth-collections/user [get]
func GetCurrentUser(c *fiber.Ctx) error {
	user := middleware.GetAppUserFromContext(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized - valid JWT token required",
		})
	}

	return c.JSON(dto.AppUserResponse{
		ID:        user.ID,
		ClientID:  user.ClientID,
		Email:     user.Email,
		Data:      user.Data,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Roles:     user.Roles,
	})
}

// UpdateCurrentUser updates the currently authenticated user's data
// @Summary Update current app user
// @Description Update the currently authenticated app user's details
// @Tags App Client
// @Accept json
// @Produce json
// @Param user body dto.AppUserUpdateRequest true "User update data"
// @Success 200 {object} dto.AppUserResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security BearerAuth
// @Router /auth-collections/user [patch]
func UpdateCurrentUser(c *fiber.Ctx) error {
	user := middleware.GetAppUserFromContext(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized - valid JWT token required",
		})
	}

	var req dto.AppUserUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	// Build only the fields that changed
	updates := map[string]interface{}{}

	if req.Email != nil {
		user.Email = *req.Email
		updates["email"] = user.Email
	}

	if req.Password != nil {
		if err := user.SetPassword(*req.Password); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": "Failed to update password",
			})
		}
		updates["password"] = user.Password
	}

	if req.Data != nil {
		user.Data = models.JSONMap(*req.Data)
		updates["data"] = user.Data
	}

	if len(updates) == 0 {
		return c.JSON(dto.AppUserToResponse(user))
	}

	if err := database.DB.Model(user).Updates(updates).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to update user",
		})
	}

	return c.JSON(dto.AppUserToResponse(user))
}

// LoginWithGoogle initiates Google OAuth flow (Method 1: Redirect)
// @Summary Initiate Google OAuth login
// @Description Get Google OAuth URL for authentication
// @Tags App Client
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /auth-collections/login-google [get]
func LoginWithGoogle(c *fiber.Ctx) error {
	project := c.Locals("project").(*models.Project)

	// Get Google OAuth integration settings
	config, err := getGoogleOAuthConfig(project.ID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err.Error(),
		})
	}

	// Generate state for CSRF protection
	state := generateRandomState()

	// Build OAuth URL
	params := url.Values{}
	params.Add("client_id", config.ClientID)
	params.Add("redirect_uri", config.RedirectURL)
	params.Add("response_type", "code")
	params.Add("scope", "openid email profile")
	params.Add("state", state)
	params.Add("access_type", "offline")
	params.Add("prompt", "consent")

	authURL := fmt.Sprintf("%s?%s", GoogleAuthURL, params.Encode())

	return c.JSON(fiber.Map{
		"success": true,
		"url":     authURL,
		"state":   state,
	})
}

// VerifyGoogleToken handles frontend token verification (Method 2: Frontend Token)
// @Summary Verify Google token from frontend
// @Description Verify Google access token obtained by frontend
// @Tags App Client
// @Accept json
// @Produce json
// @Param token body map[string]string true "Google access token"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /auth-collections/verify-google-token [post]
func VerifyGoogleToken(c *fiber.Ctx) error {
	project := c.Locals("project").(*models.Project)

	var req struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if req.AccessToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "access_token is required",
		})
	}

	// Get OAuth config to verify integration is enabled
	_, err := getGoogleOAuthConfig(project.ID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err.Error(),
		})
	}

	// Get user info from Google using access token
	userInfo, err := getUserInfoFromGoogle(req.AccessToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid Google token",
		})
	}

	// Validate email
	if userInfo.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "No email provided by Google",
		})
	}

	// Handle user authentication/registration
	appUser, isNewUser, err := handleGoogleUser(project.ID, userInfo)
	if err != nil {
		if strings.Contains(err.Error(), "already registered") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error":   true,
				"message": "Email already registered with password. Please login with password.",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to authenticate user",
		})
	}

	// Generate JWT token
	jwtToken, err := services.CreateAppUserToken(appUser)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to generate token",
		})
	}

	return c.JSON(fiber.Map{
		"success":  true,
		"token":    jwtToken,
		"new_user": isNewUser,
		"user": fiber.Map{
			"id":    appUser.ID,
			"email": appUser.Email,
			"data":  appUser.Data,
		},
	})
}

// Helper functions for Google OAuth

// getGoogleOAuthConfig retrieves OAuth config from project integration
func getGoogleOAuthConfig(projectID string) (*GoogleOAuthConfig, error) {
	var projectIntegration models.ProjectIntegration
	err := database.DB.Where(
		"project_id = ? AND integration_id = ? AND is_enabled = ?",
		projectID,
		GoogleOAuthIntegrationID,
		true,
	).First(&projectIntegration).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("Google OAuth integration is not enabled for this project")
		}
		return nil, fmt.Errorf("failed to fetch integration settings")
	}

	config := &GoogleOAuthConfig{}

	// Parse config JSON
	if clientID, ok := projectIntegration.Config["GOOGLE_CLIENT_ID"].(string); ok {
		config.ClientID = clientID
	} else {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID not configured")
	}

	if clientSecret, ok := projectIntegration.Config["GOOGLE_CLIENT_SECRET"].(string); ok {
		config.ClientSecret = clientSecret
	} else {
		return nil, fmt.Errorf("GOOGLE_CLIENT_SECRET not configured")
	}

	// Set redirect URL (with default)
	if redirectURL, ok := projectIntegration.Config["GOOGLE_REDIRECT_URL"].(string); ok {
		config.RedirectURL = redirectURL
	} else {
		config.RedirectURL = fmt.Sprintf("https://cocobase.pxxl.click/auth-collections/auth-google-redirect/%s", projectID)
	}

	// Complete URL is required
	if completeURL, ok := projectIntegration.Config["GOOGLE_COMPLETE_URL"].(string); ok {
		config.CompleteURL = completeURL
	} else {
		return nil, fmt.Errorf("GOOGLE_COMPLETE_URL not configured")
	}

	return config, nil
}

// exchangeCodeForToken exchanges authorization code for access token
func exchangeCodeForToken(code string, config *GoogleOAuthConfig) (*GoogleTokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", config.ClientID)
	data.Set("client_secret", config.ClientSecret)
	data.Set("redirect_uri", config.RedirectURL)
	data.Set("grant_type", "authorization_code")

	resp, err := http.PostForm(GoogleTokenURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var token GoogleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	return &token, nil
}

// getUserInfoFromGoogle gets user info from Google
func getUserInfoFromGoogle(accessToken string) (*GoogleUserInfo, error) {
	req, err := http.NewRequest("GET", GoogleUserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user info from Google")
	}

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// handleGoogleUser handles user authentication/registration
func handleGoogleUser(projectID string, userInfo *GoogleUserInfo) (*models.AppUser, bool, error) {
	var existingUser models.AppUser
	err := database.DB.Where(
		"email = ? AND client_id = ?",
		userInfo.Email,
		projectID,
	).First(&existingUser).Error

	if err == nil {
		// User exists
		if existingUser.OAuthID == nil || *existingUser.OAuthID == "" {
			// User registered with password, not OAuth
			return nil, false, fmt.Errorf("email already registered with password")
		}

		// User exists and used OAuth before - return existing user
		return &existingUser, false, nil
	}

	if err != gorm.ErrRecordNotFound {
		// Database error
		return nil, false, err
	}

	// Create new user
	username := strings.ReplaceAll(userInfo.Name, " ", "")
	if username == "" {
		username = fmt.Sprintf("user_%s", userInfo.Sub[:8])
	}

	newUser := &models.AppUser{
		Email:    userInfo.Email,
		ClientID: projectID,
		OAuthID:  &userInfo.Sub,
		Password: userInfo.Email, // Dummy password for OAuth users
		Data: map[string]interface{}{
			"username": username,
			"name":     userInfo.Name,
			"picture":  userInfo.Picture,
		},
	}

	if err := database.DB.Create(newUser).Error; err != nil {
		return nil, false, err
	}

	return newUser, true, nil
}

// generateRandomState generates random state for CSRF protection
func generateRandomState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
