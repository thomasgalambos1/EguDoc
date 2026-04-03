package rbac_test

import (
	"testing"

	"github.com/eguilde/egudoc/internal/rbac"
	"github.com/google/uuid"
)

// TestCheckContextBuildsCorrectly verifies that CheckContext fields are set correctly.
func TestCheckContextBuildsCorrectly(t *testing.T) {
	instID := uuid.New()
	compID := uuid.New()
	check := rbac.CheckContext{
		UserSubject:    "user-123",
		InstitutionID:  &instID,
		CompartimentID: &compID,
	}
	if check.UserSubject != "user-123" {
		t.Errorf("unexpected subject: %s", check.UserSubject)
	}
	if *check.InstitutionID != instID {
		t.Error("institution ID not set correctly")
	}
}
