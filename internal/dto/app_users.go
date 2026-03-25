package dto

import "github.com/patrick/cocobase/internal/models"

// AppUserLoginRequest represents login credentials
// @Description Login credentials for app user authentication
type AppUserLoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"user@example.com"`
	Password string `json:"password" validate:"required" example:"password123"`
}

// AppUserSignupRequest represents signup data
// @Description Signup data for creating a new app user
type AppUserSignupRequest struct {
	Email    string                 `json:"email" validate:"required,email" example:"user@example.com"`
	Password string                 `json:"password" validate:"required,min=6" example:"password123"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

// AppUserUpdateRequest represents user update data
// @Description Fields to update on the authenticated user
type AppUserUpdateRequest struct {
	Email    *string                 `json:"email,omitempty" validate:"omitempty,email" example:"new@example.com"`
	Password *string                 `json:"password,omitempty" validate:"omitempty,min=6" example:"newpassword123"`
	Data     *map[string]interface{} `json:"data,omitempty"`
}

// AppUserResponse represents user data in responses
// @Description App user data returned in API responses
type AppUserResponse struct {
	ID              string                 `json:"id" example:"uuid-here"`
	ClientID        string                 `json:"client_id" example:"project-uuid"`
	Email           string                 `json:"email" example:"user@example.com"`
	Data            map[string]interface{} `json:"data"`
	CreatedAt       string                 `json:"created_at" example:"2024-01-01T00:00:00Z"`
	Roles           []string               `json:"roles"`
	EmailVerified   bool                   `json:"email_verified" example:"false"`
	EmailVerifiedAt *string                `json:"email_verified_at"`
	PhoneNumber     *string                `json:"phone_number"`
	PhoneVerifiedAt *string                `json:"phone_verified_at"`
}

// TokenResponse represents JWT token response
// @Description JWT token and user data returned after successful authentication
type TokenResponse struct {
	AccessToken string          `json:"access_token" example:"eyJhbGci..."`
	User        AppUserResponse `json:"user"`
	Requires2FA bool            `json:"requires_2fa" example:"false"`
	Message     string          `json:"message,omitempty" example:""`
}

func AppUserToResponse(u *models.AppUser) AppUserResponse {
	return AppUserResponse{
		ID:            u.ID,
		ClientID:      u.ClientID,
		Email:         u.Email,
		Data:          u.Data,
		CreatedAt:     u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Roles:         u.Roles,
		EmailVerified: u.EmailVerified,
		EmailVerifiedAt: func() *string {
			if u.EmailVerifiedAt != nil {
				str := u.EmailVerifiedAt.Format("2006-01-02T15:04:05Z07:00")
				return &str
			}
			return nil
		}(),
		PhoneNumber: u.PhoneNumber,
		PhoneVerifiedAt: func() *string {
			if u.PhoneVerifiedAt != nil {
				str := u.PhoneVerifiedAt.Format("2006-01-02T15:04:05Z07:00")
				return &str
			}
			return nil
		}(),
	}
}
