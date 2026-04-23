package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/patrick/cocobase/pkg/config"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/dto"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
	fnservice "github.com/patrick/cocobase/internal/services/functions"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────
// Google OAuth
// ─────────────────────────────────────────

const (
	GoogleOAuthIntegrationID = "046deb41-47b3-403d-aee8-b80ccb80a87e"
	GoogleTokenInfoURL       = "https://oauth2.googleapis.com/tokeninfo"
	GoogleUserInfoURL        = "https://www.googleapis.com/oauth2/v2/userinfo"
	GoogleAuthURL            = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL           = "https://oauth2.googleapis.com/token"
)

type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
}

type GoogleOAuthConfig struct {
	ClientID     string `json:"GOOGLE_CLIENT_ID"`
	ClientSecret string `json:"GOOGLE_CLIENT_SECRET"`
}

// ─────────────────────────────────────────
// GitHub OAuth
// ─────────────────────────────────────────

const (
	GitHubOAuthIntegrationID = "cee2caf5-647d-46b9-bd6b-9f0ed80e74fb"
	GitHubUserInfoURL        = "https://api.github.com/user"
	GitHubUserEmailsURL      = "https://api.github.com/user/emails"
	GitHubTokenURL           = "https://github.com/login/oauth/access_token"
)

type GitHubOAuthConfig struct {
	ClientID     string `json:"GITHUB_CLIENT_ID"`
	ClientSecret string `json:"GITHUB_CLIENT_SECRET"`
}

type GitHubUserInfo struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// ─────────────────────────────────────────
// Apple OAuth
// ─────────────────────────────────────────

const (
	ApplePublicKeysURL = "https://appleid.apple.com/auth/keys"
)

type AppleJWKS struct {
	Keys []AppleJWK `json:"keys"`
}

type AppleJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type AppleUserInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
}

// ─────────────────────────────────────────
// Shared helpers
// ─────────────────────────────────────────

func generateRandomState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// findOrCreateOAuthUser finds existing OAuth user or creates new one.
// Returns user, isNewUser, error.
func findOrCreateOAuthUser(projectID, email, oauthID, provider, name, picture string) (*models.AppUser, bool, error) {
	var existing models.AppUser
	err := database.DB.Where("email = ? AND client_id = ?", email, projectID).First(&existing).Error

	if err == nil {
		// User found
		if existing.OAuthID == nil || *existing.OAuthID == "" {
			return nil, false, fmt.Errorf("email already registered with password")
		}
		return &existing, false, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, false, err
	}

	// Create new user
	data := map[string]interface{}{
		"name":    name,
		"picture": picture,
	}

	newUser := &models.AppUser{
		Email:         email,
		ClientID:      projectID,
		OAuthID:       &oauthID,
		OAuthProvider: &provider,
		Password:      email, // placeholder
		Data:          data,
	}

	// Fetch project name for hooks (best-effort; skip hooks on error)
	var proj models.Project
	if err := database.DB.Select("id, name").First(&proj, "id = ?", projectID).Error; err == nil {
		doc := fnservice.AppUserToHookDoc(newUser)
		if cancelled, msg, mutated := fnservice.DispatchAppUserHook(
			models.HookBeforeCreate, projectID, proj.Name, doc, nil, nil,
		); cancelled {
			return nil, false, fmt.Errorf("hook cancelled: %s", msg)
		} else {
			fnservice.ApplyHookDocToUser(mutated, newUser)
		}
	}

	if err := database.DB.Create(newUser).Error; err != nil {
		return nil, false, err
	}

	// afterCreate hook — fire-and-forget
	if proj.ID != "" {
		go fnservice.DispatchAppUserHook(
			models.HookAfterCreate, projectID, proj.Name,
			fnservice.AppUserToHookDoc(newUser), newUser, nil,
		)
	}

	return newUser, true, nil
}

func getIntegrationConfig(projectID, integrationID string) (models.JSONMap, error) {
	var pi models.ProjectIntegration
	err := database.DB.Where(
		"project_id = ? AND integration_id = ? AND is_enabled = ?",
		projectID, integrationID, true,
	).First(&pi).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("integration not enabled for this project")
		}
		return nil, fmt.Errorf("failed to fetch integration settings")
	}
	return pi.Config, nil
}

// ─────────────────────────────────────────
// Basic Auth Handlers
// ─────────────────────────────────────────

// UserLogin authenticates an app user and returns a JWT token
// @Summary App user login
// @Tags App Client
// @Accept json
// @Produce json
// @Param credentials body dto.AppUserLoginRequest true "Login credentials"
// @Success 200 {object} dto.TokenResponse
// @Security ApiKeyAuth
// @Router /auth-collections/login [post]
func UserLogin(c *fiber.Ctx) error {

	var req dto.AppUserLoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}

	var user models.AppUser
	if err := database.DB.Where("client_id = ? AND email = ?", instanceID(), req.Email).First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Account with this email does not exist"})
	}

	if !user.ComparePassword(req.Password) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid password value"})
	}

	token, err := services.CreateAppUserToken(&user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to generate token"})
	}

	return c.JSON(dto.TokenResponse{AccessToken: token, User: dto.AppUserToResponse(&user)})
}

// UserSignup creates a new app user and returns a JWT token
// @Summary App user signup
// @Tags App Client
// @Accept json
// @Produce json
// @Param user body dto.AppUserSignupRequest true "User data"
// @Success 200 {object} dto.TokenResponse
// @Security ApiKeyAuth
// @Router /auth-collections/signup [post]
func UserSignup(c *fiber.Ctx) error {

	var req dto.AppUserSignupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}

	var existingUser models.AppUser
	if err := database.DB.Where("client_id = ? AND email = ?", instanceID(), req.Email).First(&existingUser).Error; err == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "User with this email already exists"})
	}

	if req.Data == nil {
		req.Data = make(map[string]interface{})
	}

	user := models.AppUser{
		ClientID: instanceID(),
		Email:    req.Email,
		Data:     req.Data,
		Roles:    models.StringArray{},
	}

	if err := user.SetPassword(req.Password); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to hash password"})
	}

	// beforeCreate hook — JS can mutate doc fields or cancel
	doc := fnservice.AppUserToHookDoc(&user)
	if cancelled, msg, mutated := fnservice.DispatchAppUserHook(
		models.HookBeforeCreate, instanceID(), "default", doc, nil, BroadcastToProject,
	); cancelled {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": msg})
	} else {
		fnservice.ApplyHookDocToUser(mutated, &user)
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to create user"})
	}

	// afterCreate hook — fire-and-forget
	go fnservice.DispatchAppUserHook(
		models.HookAfterCreate, instanceID(), "default",
		fnservice.AppUserToHookDoc(&user), &user, BroadcastToProject,
	)

	token, err := services.CreateAppUserToken(&user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to generate token"})
	}

	return c.JSON(dto.TokenResponse{AccessToken: token, User: dto.AppUserToResponse(&user)})
}

// ListAllUsers returns all users for a project
// @Summary List app users
// @Tags App Client
// @Produce json
// @Success 200 {array} dto.AppUserResponse
// @Security ApiKeyAuth
// @Router /auth-collections/users [get]
func ListAllUsers(c *fiber.Ctx) error {

	limit := c.QueryInt("limit", 100)
	offset := c.QueryInt("offset", 0)
	if limit > 1000 {
		limit = 1000
	}

	var total int64
	database.DB.Model(&models.AppUser{}).Where("client_id = ?", instanceID()).Count(&total)

	var users []models.AppUser
	if err := database.DB.Where("client_id = ?", instanceID()).Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to fetch users"})
	}

	data := make([]dto.AppUserResponse, len(users))
	for i, user := range users {
		data[i] = dto.AppUserToResponse(&user)
	}

	return c.JSON(fiber.Map{
		"data":     data,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"has_more": int64(offset+limit) < total,
	})
}

// GetUserByID returns a specific user by ID
// @Summary Get app user by ID
// @Tags App Client
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} dto.AppUserResponse
// @Security ApiKeyAuth
// @Router /auth-collections/users/{id} [get]
func GetUserByID(c *fiber.Ctx) error {

	userID := c.Params("id")
	var user models.AppUser
	if err := database.DB.Where("client_id = ? AND id = ?", instanceID(), userID).First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "User not found"})
	}

	return c.JSON(dto.AppUserToResponse(&user))
}

// GetCurrentUser returns the currently authenticated user
// @Summary Get current app user
// @Tags App Client
// @Produce json
// @Success 200 {object} dto.AppUserResponse
// @Security BearerAuth
// @Router /auth-collections/user [get]
func GetCurrentUser(c *fiber.Ctx) error {
	user := middleware.GetAppUserFromContext(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Unauthorized"})
	}
	return c.JSON(dto.AppUserToResponse(user))
}

// UpdateCurrentUser updates the currently authenticated user's data
// @Summary Update current app user
// @Tags App Client
// @Accept json
// @Produce json
// @Param user body dto.AppUserUpdateRequest true "User update data"
// @Success 200 {object} dto.AppUserResponse
// @Security BearerAuth
// @Router /auth-collections/user [patch]
func UpdateCurrentUser(c *fiber.Ctx) error {
	user := middleware.GetAppUserFromContext(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Unauthorized"})
	}

	var req struct {
		dto.AppUserUpdateRequest
		Override bool `json:"override"` // if true, Data fully replaces existing data
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}

	updates := map[string]interface{}{}

	if req.Email != nil {
		user.Email = *req.Email
		updates["email"] = user.Email
	}

	if req.Password != nil {
		if err := user.SetPassword(*req.Password); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to update password"})
		}
		updates["password"] = user.Password
	}

	if req.Data != nil {
		if req.Override {
			user.Data = models.JSONMap(*req.Data)
		} else {
			// Merge: new keys overwrite existing, existing keys not in new data are kept
			for k, v := range *req.Data {
				user.Data[k] = v
			}
		}
		updates["data"] = user.Data
	}

	if len(updates) == 0 {
		return c.JSON(dto.AppUserToResponse(user))
	}

	// beforeUpdate hook — JS can mutate doc fields or cancel
	var proj models.Project
	if err := database.DB.Select("id, name").First(&proj, "id = ?", user.ClientID).Error; err == nil {
		doc := fnservice.AppUserToHookDoc(user)
		if cancelled, msg, mutated := fnservice.DispatchAppUserHook(
			models.HookBeforeUpdate, proj.ID, proj.Name, doc, user, BroadcastToProject,
		); cancelled {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": msg})
		} else {
			fnservice.ApplyHookDocToUser(mutated, user)
			// Rebuild updates map to reflect any hook mutations
			updates = map[string]interface{}{
				"email": user.Email,
				"data":  user.Data,
				"roles": user.Roles,
			}
			if user.Password != "" {
				updates["password"] = user.Password
			}
		}
	}

	if err := database.DB.Model(user).Updates(updates).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to update user"})
	}

	// afterUpdate hook — fire-and-forget
	if proj.ID != "" {
		go fnservice.DispatchAppUserHook(
			models.HookAfterUpdate, proj.ID, proj.Name,
			fnservice.AppUserToHookDoc(user), user, BroadcastToProject,
		)
	}

	return c.JSON(dto.AppUserToResponse(user))
}

// ─────────────────────────────────────────
// Google OAuth
// ─────────────────────────────────────────

// VerifyGoogleToken verifies a Google ID token or access token
// @Summary Verify Google token
// @Tags App Client
// @Accept json
// @Produce json
// @Param body body map[string]string true "Google id_token or access_token"
// @Success 200 {object} dto.TokenResponse
// @Security ApiKeyAuth
// @Router /auth-collections/google-verify [post]
func VerifyGoogleToken(c *fiber.Ctx) error {

	var req struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}

	if req.IDToken == "" && req.AccessToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "id_token or access_token is required"})
	}

	// Check integration is enabled (config optional - Google token self-validates)
	cfg, _ := getIntegrationConfig(instanceID(), GoogleOAuthIntegrationID)

	var userInfo *GoogleUserInfo
	var err error

	if req.IDToken != "" {
		userInfo, err = verifyGoogleIDToken(req.IDToken, cfg)
	} else {
		userInfo, err = getUserInfoFromGoogle(req.AccessToken)
	}

	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Invalid Google token: " + err.Error()})
	}

	if userInfo.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "No email provided by Google"})
	}

	appUser, isNewUser, err := findOrCreateOAuthUser(instanceID(), userInfo.Email, userInfo.Sub, "google", userInfo.Name, userInfo.Picture)
	if err != nil {
		if strings.Contains(err.Error(), "already registered") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": true, "message": "Email already registered with password. Please login with password."})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to authenticate user"})
	}

	token, err := services.CreateAppUserToken(appUser)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to generate token"})
	}

	_ = isNewUser // not included in response to match Python
	return c.JSON(dto.TokenResponse{AccessToken: token, User: dto.AppUserToResponse(appUser)})
}

// verifyGoogleIDToken verifies a Google ID token via tokeninfo endpoint
func verifyGoogleIDToken(idToken string, cfg models.JSONMap) (*GoogleUserInfo, error) {
	resp, err := http.Get(GoogleTokenInfoURL + "?id_token=" + idToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("invalid token: %s", string(body))
	}

	var info struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &GoogleUserInfo{
		Sub:           info.Sub,
		Email:         info.Email,
		EmailVerified: info.EmailVerified == "true",
		Name:          info.Name,
		Picture:       info.Picture,
		GivenName:     info.GivenName,
		FamilyName:    info.FamilyName,
	}, nil
}

// getUserInfoFromGoogle gets user info using an access token
func getUserInfoFromGoogle(accessToken string) (*GoogleUserInfo, error) {
	req, err := http.NewRequest("GET", GoogleUserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

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

// getGoogleOAuthConfig retrieves OAuth config from project integration (kept for redirect flow)
func getGoogleOAuthConfig(projectID string) (*GoogleOAuthConfig, error) {
	cfg, err := getIntegrationConfig(projectID, GoogleOAuthIntegrationID)
	if err != nil {
		return nil, err
	}

	config := &GoogleOAuthConfig{}
	if v, ok := cfg["GOOGLE_CLIENT_ID"].(string); ok {
		config.ClientID = v
	} else {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID not configured")
	}
	if v, ok := cfg["GOOGLE_CLIENT_SECRET"].(string); ok {
		config.ClientSecret = v
	}
	return config, nil
}

// LoginWithGoogle returns a Google OAuth redirect URL
// @Summary Initiate Google OAuth login
// @Tags App Client
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /auth-collections/login-google [get]
func LoginWithGoogle(c *fiber.Ctx) error {

	config, err := getGoogleOAuthConfig(instanceID())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": err.Error()})
	}

	state := generateRandomState()
	params := url.Values{}
	params.Add("client_id", config.ClientID)
	params.Add("response_type", "code")
	params.Add("scope", "openid email profile")
	params.Add("state", state)
	params.Add("access_type", "offline")

	authURL := fmt.Sprintf("%s?%s", GoogleAuthURL, params.Encode())
	return c.JSON(fiber.Map{"success": true, "url": authURL, "state": state})
}

// ─────────────────────────────────────────
// GitHub OAuth
// ─────────────────────────────────────────

// VerifyGitHubToken verifies a GitHub access token or exchanges a code
// @Summary Verify GitHub token
// @Tags App Client
// @Accept json
// @Produce json
// @Param body body map[string]string true "GitHub access_token or code"
// @Success 200 {object} dto.TokenResponse
// @Security ApiKeyAuth
// @Router /auth-collections/github-verify [post]
func VerifyGitHubToken(c *fiber.Ctx) error {

	var req struct {
		AccessToken string `json:"access_token"`
		Code        string `json:"code"`
		RedirectURI string `json:"redirect_uri"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}

	cfg, err := getIntegrationConfig(instanceID(), GitHubOAuthIntegrationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "GitHub integration not enabled for this project"})
	}

	clientID, _ := cfg["GITHUB_CLIENT_ID"].(string)
	clientSecret, _ := cfg["GITHUB_CLIENT_SECRET"].(string)

	accessToken := req.AccessToken

	// If code provided, exchange it for access token
	if accessToken == "" && req.Code != "" {
		if clientID == "" || clientSecret == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "GitHub client_id and client_secret required for code exchange"})
		}
		accessToken, err = exchangeGitHubCode(req.Code, req.RedirectURI, clientID, clientSecret)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Failed to exchange GitHub code: " + err.Error()})
		}
	}

	if accessToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "access_token or code is required"})
	}

	userInfo, err := getGitHubUserInfo(accessToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Invalid GitHub token: " + err.Error()})
	}

	if userInfo.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "No public email on GitHub account. Please add a public email."})
	}

	oauthID := fmt.Sprintf("%d", userInfo.ID)
	appUser, isNewUser, err := findOrCreateOAuthUser(instanceID(), userInfo.Email, oauthID, "github", userInfo.Name, userInfo.AvatarURL)
	if err != nil {
		if strings.Contains(err.Error(), "already registered") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": true, "message": "Email already registered with password. Please login with password."})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to authenticate user"})
	}

	token, err := services.CreateAppUserToken(appUser)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to generate token"})
	}

	_ = isNewUser
	return c.JSON(dto.TokenResponse{AccessToken: token, User: dto.AppUserToResponse(appUser)})
}

func exchangeGitHubCode(code, redirectURI, clientID, clientSecret string) (string, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	if redirectURI != "" {
		data.Set("redirect_uri", redirectURI)
	}

	req, err := http.NewRequest("POST", GitHubTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("%s", result.Error)
	}
	return result.AccessToken, nil
}

func getGitHubUserInfo(accessToken string) (*GitHubUserInfo, error) {
	req, err := http.NewRequest("GET", GitHubUserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var user GitHubUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	// If email is not public, fetch from emails endpoint
	if user.Email == "" {
		email, err := getGitHubPrimaryEmail(accessToken)
		if err == nil {
			user.Email = email
		}
	}

	return &user, nil
}

func getGitHubPrimaryEmail(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", GitHubUserEmailsURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no primary verified email")
}

// ─────────────────────────────────────────
// Apple OAuth
// ─────────────────────────────────────────

// VerifyAppleToken verifies an Apple ID token
// @Summary Verify Apple token
// @Tags App Client
// @Accept json
// @Produce json
// @Param body body map[string]string true "Apple id_token"
// @Success 200 {object} dto.TokenResponse
// @Security ApiKeyAuth
// @Router /auth-collections/apple-verify [post]
func VerifyAppleToken(c *fiber.Ctx) error {

	var req struct {
		IDToken string `json:"id_token"`
		User    *struct {
			Name *struct {
				FirstName string `json:"firstName"`
				LastName  string `json:"lastName"`
			} `json:"name,omitempty"`
		} `json:"user,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}

	if req.IDToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "id_token is required"})
	}

	userInfo, err := verifyAppleIDToken(req.IDToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Invalid Apple token: " + err.Error()})
	}

	if userInfo.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "No email provided by Apple"})
	}

	// Apple only sends name on first auth
	name := ""
	if req.User != nil && req.User.Name != nil {
		name = strings.TrimSpace(req.User.Name.FirstName + " " + req.User.Name.LastName)
	}

	appUser, isNewUser, err := findOrCreateOAuthUser(instanceID(), userInfo.Email, userInfo.Sub, "apple", name, "")
	if err != nil {
		if strings.Contains(err.Error(), "already registered") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": true, "message": "Email already registered with password. Please login with password."})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to authenticate user"})
	}

	token, err := services.CreateAppUserToken(appUser)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to generate token"})
	}

	_ = isNewUser
	return c.JSON(dto.TokenResponse{AccessToken: token, User: dto.AppUserToResponse(appUser)})
}

func verifyAppleIDToken(idToken string) (*AppleUserInfo, error) {
	// Fetch Apple public keys
	resp, err := http.Get(ApplePublicKeysURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Apple public keys: %w", err)
	}
	defer resp.Body.Close()

	var jwks AppleJWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode Apple public keys: %w", err)
	}

	// Parse token header to get kid
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT header")
	}

	var header struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("failed to parse JWT header")
	}

	// Find matching key
	var matchingKey *AppleJWK
	for i, key := range jwks.Keys {
		if key.Kid == header.Kid {
			matchingKey = &jwks.Keys[i]
			break
		}
	}
	if matchingKey == nil {
		return nil, fmt.Errorf("no matching Apple public key found")
	}

	// Build RSA public key
	pubKey, err := buildRSAPublicKey(matchingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to build RSA public key: %w", err)
	}

	// Parse and verify JWT
	token, err := jwt.Parse(idToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return pubKey, nil
	}, jwt.WithIssuer("https://appleid.apple.com"))

	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)

	return &AppleUserInfo{Sub: sub, Email: email}, nil
}

func buildRSAPublicKey(jwk *AppleJWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, err
	}

	n := new(big.Int).SetBytes(nBytes)
	eInt := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{N: n, E: int(eInt.Int64())}, nil
}

// ─────────────────────────────────────────
// Password Reset
// ─────────────────────────────────────────

// ForgotPassword sends a password reset link (delegates email to mailer service)
// @Summary Forgot password
// @Tags App Client
// @Accept json
// @Produce json
// @Param body body map[string]string true "email"
// @Success 200 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /auth-collections/forgot-password [post]
func ForgotPassword(c *fiber.Ctx) error {

	var req struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&req); err != nil || req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "email is required"})
	}

	// Check user exists (don't reveal if not found - security)
	var user models.AppUser
	if err := database.DB.Where("client_id = ? AND email = ?", instanceID(), req.Email).First(&user).Error; err != nil {
		// Return success regardless to prevent email enumeration
		return c.JSON(fiber.Map{"success": true, "message": "If that email exists, a reset link has been sent"})
	}

	// Generate reset token
	token := uuid.New().String()
	expiresAt := time.Now().Add(1 * time.Hour)

	// Invalidate previous tokens for this user
	database.DB.Model(&models.PasswordResetToken{}).
		Where("user_id = ? AND is_used = ?", user.ID, false).
		Update("is_used", true)

	resetToken := models.PasswordResetToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: expiresAt,
	}
	if err := database.DB.Create(&resetToken).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to create reset token"})
	}

	frontendURL := config.AppConfig.FrontendURL
	go services.SendPasswordResetEmail(nil, user.Email, token, frontendURL)

	return c.JSON(fiber.Map{
		"message": "If that email exists, a reset link has been sent",
	})
}

// ResetPassword resets a user's password using a valid token
// @Summary Reset password
// @Tags App Client
// @Accept json
// @Produce json
// @Param body body map[string]string true "token and new password"
// @Success 200 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /auth-collections/reset-password [post]
func ResetPassword(c *fiber.Ctx) error {

	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil || req.Token == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "token and password are required"})
	}

	// Find valid token
	var resetToken models.PasswordResetToken
	if err := database.DB.Where("token = ? AND is_used = ?", req.Token, false).First(&resetToken).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid or expired reset token"})
	}

	if time.Now().After(resetToken.ExpiresAt) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Reset token has expired"})
	}

	// Find user and ensure they belong to this project
	var user models.AppUser
	if err := database.DB.Where("id = ? AND client_id = ?", resetToken.UserID, instanceID()).First(&user).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "User not found"})
	}

	if err := user.SetPassword(req.Password); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to hash password"})
	}

	if err := database.DB.Model(&user).Update("password", user.Password).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to update password"})
	}

	// Mark token as used
	database.DB.Model(&resetToken).Update("is_used", true)

	return c.JSON(fiber.Map{"message": "Password successfully reset"})
}

// ResetPasswordPage returns a simple HTML password reset form
// @Summary Reset password page
// @Tags App Client
// @Produce html
// @Param token query string true "Reset token"
// @Router /auth-collections/reset-password-page [get]
func ResetPasswordPage(c *fiber.Ctx) error {
	token := c.Query("token", "")
	c.Set("Content-Type", "text/html")
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>Reset Password</title>
<style>body{font-family:sans-serif;max-width:400px;margin:80px auto;padding:0 20px}
input{width:100%%;padding:10px;margin:8px 0;box-sizing:border-box;border:1px solid #ccc;border-radius:4px}
button{width:100%%;padding:12px;background:#4f46e5;color:white;border:none;border-radius:4px;cursor:pointer;font-size:16px}
.msg{padding:10px;border-radius:4px;margin-top:10px}</style></head>
<body>
<h2>Reset Password</h2>
<form id="form">
<input type="hidden" id="token" value="%s">
<input type="password" id="password" placeholder="New password" required minlength="6">
<input type="password" id="confirm" placeholder="Confirm password" required>
<button type="submit">Reset Password</button>
<div id="msg" class="msg"></div>
</form>
<script>
document.getElementById('form').onsubmit = async function(e) {
  e.preventDefault();
  const pw = document.getElementById('password').value;
  const cf = document.getElementById('confirm').value;
  const msg = document.getElementById('msg');
  if (pw !== cf) { msg.style.background='#fee2e2'; msg.innerText='Passwords do not match'; return; }
  const r = await fetch(window.location.pathname.replace('reset-password-page','reset-password'), {
    method:'POST', headers:{'Content-Type':'application/json'},
    body: JSON.stringify({token: document.getElementById('token').value, password: pw})
  });
  const d = await r.json();
  if (d.success) { msg.style.background='#dcfce7'; msg.innerText='Password reset! You can now login.'; document.getElementById('form').reset(); }
  else { msg.style.background='#fee2e2'; msg.innerText=d.message||'Error'; }
};
</script>
</body></html>`, token)
	return c.SendString(html)
}

// ─────────────────────────────────────────
// Email Verification
// ─────────────────────────────────────────

// SendVerificationEmail sends an email verification token
// @Summary Send verification email
// @Tags App Client
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /auth-collections/verify-email/send [post]
func SendVerificationEmail(c *fiber.Ctx) error {
	user := middleware.GetAppUserFromContext(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Unauthorized"})
	}

	if user.EmailVerified {
		return c.JSON(fiber.Map{"message": "Email already verified"})
	}

	return issueEmailVerificationToken(c, user)
}

// VerifyEmail verifies an email using a token
// @Summary Verify email
// @Tags App Client
// @Accept json
// @Produce json
// @Param body body map[string]string true "token"
// @Success 200 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /auth-collections/verify-email/verify [post]
func VerifyEmail(c *fiber.Ctx) error {

	var req struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&req); err != nil || req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "token is required"})
	}

	var vToken models.EmailVerificationToken
	if err := database.DB.Where("token = ? AND client_id = ? AND is_used = ?", req.Token, instanceID(), false).First(&vToken).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid or expired verification token"})
	}

	if time.Now().After(vToken.ExpiresAt) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Verification token has expired"})
	}

	now := time.Now()
	if err := database.DB.Model(&models.AppUser{}).Where("id = ?", vToken.UserID).
		Updates(map[string]interface{}{"email_verified": true, "email_verified_at": now}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to verify email"})
	}

	database.DB.Model(&vToken).Update("is_used", true)

	return c.JSON(fiber.Map{
		"message":       "Email verified successfully!",
		"email_verified": true,
		"verified_at":   now,
	})
}

// ResendVerificationEmail resends a verification email
// @Summary Resend verification email
// @Tags App Client
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /auth-collections/verify-email/resend [post]
func ResendVerificationEmail(c *fiber.Ctx) error {
	user := middleware.GetAppUserFromContext(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Unauthorized"})
	}

	if user.EmailVerified {
		return c.JSON(fiber.Map{"message": "Email already verified"})
	}

	// Invalidate previous tokens
	database.DB.Model(&models.EmailVerificationToken{}).
		Where("user_id = ? AND is_used = ?", user.ID, false).
		Update("is_used", true)

	return issueEmailVerificationToken(c, user)
}

// issueEmailVerificationToken creates and returns a verification token
func issueEmailVerificationToken(c *fiber.Ctx, user *models.AppUser) error {
	token := uuid.New().String()
	vToken := models.EmailVerificationToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		ClientID:  user.ClientID,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := database.DB.Create(&vToken).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to create verification token"})
	}

	// Load project for per-project mailer config
	var project models.Project
	if err := database.DB.First(&project, "id = ?", user.ClientID).Error; err != nil {
		// soft-fail — send without project context
		go services.SendVerificationEmail(nil, user.Email, token, config.AppConfig.FrontendURL)
	} else {
		go services.SendVerificationEmail(&project, user.Email, token, config.AppConfig.FrontendURL)
	}

	return c.JSON(fiber.Map{
		"message":          "Verification email sent successfully. Please check your inbox.",
		"expires_in_hours": 24,
	})
}
