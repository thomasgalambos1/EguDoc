//go:build tools

package main

import (
	// Keep these dependencies in go.mod for use in upcoming packages
	_ "github.com/jackc/pgx/v5"
	_ "github.com/minio/minio-go/v7"
	_ "github.com/go-jose/go-jose/v4"
	_ "github.com/google/uuid"
	_ "github.com/golang-migrate/migrate/v4"
	_ "github.com/go-chi/httprate"
)
