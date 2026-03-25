package models

import (
	"github.com/patrick/cocobase/pkg/utils"
)

// SetPassword hashes and sets the password for AppUser
func (a *AppUser) SetPassword(password string) error {
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return err
	}
	a.Password = hashedPassword
	return nil
}

// ComparePassword compares plain text password with hashed password for AppUser
func (a *AppUser) ComparePassword(password string) bool {
	return utils.VerifyPassword(password, a.Password)
}

// SetPassword hashes and sets the password for User
func (u *User) SetPassword(password string) error {
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return err
	}
	u.Password = hashedPassword
	return nil
}

// ComparePassword compares plain text password with hashed password for User
func (u *User) ComparePassword(password string) bool {
	return utils.VerifyPassword(password, u.Password)
}
