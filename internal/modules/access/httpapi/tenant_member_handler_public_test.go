package httpapi

import (
	"testing"

	"github.com/company/auto-healing/internal/modules/access/model"
	"github.com/google/uuid"
)

func TestBuildInvitationRegisterRequestDoesNotAutoAttachTenant(t *testing.T) {
	inv := &model.TenantInvitation{
		TenantID: uuid.New(),
		Email:    "invitee@example.com",
	}
	req := RegisterByInvitationRequest{
		Username:    "invitee",
		Password:    "password123",
		DisplayName: "Invitee",
	}

	registerReq := buildInvitationRegisterRequest(req, inv)
	if registerReq.TenantID != nil {
		t.Fatalf("buildInvitationRegisterRequest() tenant_id = %v, want nil", *registerReq.TenantID)
	}
	if registerReq.Email != inv.Email {
		t.Fatalf("buildInvitationRegisterRequest() email = %q, want %q", registerReq.Email, inv.Email)
	}
}
