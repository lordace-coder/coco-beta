package services

import (
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/models"
)

// PermissionType represents the type of operation
type PermissionType string

const (
	PermissionCreate PermissionType = "create"
	PermissionRead   PermissionType = "read"
	PermissionUpdate PermissionType = "update"
	PermissionDelete PermissionType = "delete"
)

// PermissionChecker handles collection permission checks
type PermissionChecker struct{}

// NewPermissionChecker creates a new permission checker
func NewPermissionChecker() *PermissionChecker {
	return &PermissionChecker{}
}

// CanAccessCollection checks if a user can perform an operation on a collection
func (pc *PermissionChecker) CanAccessCollection(
	collection *models.Collection,
	permType PermissionType,
	appUser *models.AppUser,
) error {
	// If no permissions are set, allow all access
	if collection.Permissions.Create == nil &&
		collection.Permissions.Read == nil &&
		collection.Permissions.Update == nil &&
		collection.Permissions.Delete == nil {
		return nil
	}

	// Get the permission list for the operation type
	var permList []string
	switch permType {
	case PermissionCreate:
		permList = collection.Permissions.Create
	case PermissionRead:
		permList = collection.Permissions.Read
	case PermissionUpdate:
		permList = collection.Permissions.Update
	case PermissionDelete:
		permList = collection.Permissions.Delete
	}

	// If permission list is empty, allow access
	if len(permList) == 0 {
		return nil
	}

	// If no app user and permissions are required, deny access
	if appUser == nil {
		return fiber.NewError(fiber.StatusForbidden, "Authentication required for this operation")
	}

	// Check if user has any of the required roles
	userRoles := appUser.Roles
	if userRoles == nil {
		userRoles = []string{}
	}

	for _, requiredRole := range permList {
		for _, userRole := range userRoles {
			if userRole == requiredRole || requiredRole == "*" {
				return nil
			}
		}
	}

	return fiber.NewError(fiber.StatusForbidden, "Insufficient permissions for this operation")
}

// HasRole checks if an app user has a specific role
func (pc *PermissionChecker) HasRole(appUser *models.AppUser, role string) bool {
	if appUser == nil || appUser.Roles == nil {
		return false
	}

	for _, r := range appUser.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if an app user has any of the specified roles
func (pc *PermissionChecker) HasAnyRole(appUser *models.AppUser, roles []string) bool {
	if appUser == nil || appUser.Roles == nil {
		return false
	}

	for _, requiredRole := range roles {
		for _, userRole := range appUser.Roles {
			if userRole == requiredRole {
				return true
			}
		}
	}
	return false
}
