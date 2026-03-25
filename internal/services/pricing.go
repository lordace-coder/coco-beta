package services

import (
	"log"

	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"gorm.io/gorm"
)

// GetCurrentPlan returns the current active plan for a project
// Matches Python: get_current_plan(project, db)
func GetCurrentPlan(project *models.Project) *models.Plan {
	var projectPlan models.ProjectPlan

	// Query project_subscriptions table (project_plans in Go naming)
	// Order by start_date descending to get most recent subscription
	err := database.DB.
		Preload("Plan").
		Where("project_id = ? AND is_active = ?", project.ID, true).
		Order("start_date DESC").
		First(&projectPlan).Error

	if err == nil && projectPlan.Plan != nil {
		// Plan found successfully
		return projectPlan.Plan
	}

	if err != nil && err != gorm.ErrRecordNotFound {
		log.Printf("⚠️ Error loading plan for project %s: %v", project.ID, err)
	}

	// Fallback to free plan (matches Python behavior)
	// Only log on first fallback (subsequent calls are cached)
	return GetDefaultFreePlan()
}

// GetDefaultFreePlan returns the free plan from database
// Matches Python: db.query(PricingPlan).filter_by(is_free=True).first()
func GetDefaultFreePlan() *models.Plan {
	var plan models.Plan

	// Query pricing_plans table for the free plan
	err := database.DB.Where("is_free = ? AND is_active = ?", true, true).First(&plan).Error

	if err == nil {
		return &plan
	}

	// If no free plan found, try by name
	err = database.DB.Where("name = ? AND is_active = ?", "Free", true).First(&plan).Error
	if err == nil {
		return &plan
	}

	// Fallback to unlimited hardcoded plan if database query fails
	log.Printf("⚠️ Could not load Free plan from database (is_free=true or name='Free'), using hardcoded defaults")
	maxStorage := 1000
	maxUsers := 10
	maxFunctions := 5

	return &models.Plan{
		Name:                "Free (Fallback)",
		Description:         "Default free plan",
		Price:               0,
		Currency:            "USD",
		MaxStorageMB:        &maxStorage,
		MaxUsers:            &maxUsers,
		MaxFuncExecuteTime:  &maxFunctions,
		IsActive:            true,
		IsFree:              true,
		MaxRequestsPerMonth: nil,
	}
}

// CreateDefaultPlans is disabled - pricing_plans table already exists
func CreateDefaultPlans() error {
	// Disabled - pricing_plans table already exists with data
	return nil
}
