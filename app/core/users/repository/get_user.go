package repository

import (
	"context"

	"github.com/arya-bhanu/go-doc-generator/app/core/users"
	"github.com/arya-bhanu/go-doc-generator/app/database"
)

// GetUserWithEmail looks up a user in the ops_user table by their email address.
// It returns the matching UserOps and a nil error on success.
// If no row is found it returns pgx.ErrNoRows so callers can distinguish
// "not found" from other database errors.
func GetUserWithEmail(email string) (users.UserOps, error) {
	var user users.UserOps
	err := database.DB.QueryRow(
		context.Background(),
		"SELECT id, email FROM ops_user WHERE email = $1",
		email,
	).Scan(&user.ID, &user.Email)
	if err != nil {
		return users.UserOps{}, err
	}
	return user, nil
}

// GetUserByID looks up a user in the ops_user table by their numeric ID.
// It returns the matching UserOps and a nil error on success.
// If no row is found it returns pgx.ErrNoRows.
func GetUserByID(id int) (users.UserOps, error) {
	var user users.UserOps
	err := database.DB.QueryRow(
		context.Background(),
		"SELECT id, email FROM ops_user WHERE id = $1",
		id,
	).Scan(&user.ID, &user.Email)
	if err != nil {
		return users.UserOps{}, err
	}
	return user, nil
}
