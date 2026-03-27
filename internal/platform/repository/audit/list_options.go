package audit

import (
	"time"

	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
)

type AuditLogListOptions struct {
	Page                 int
	PageSize             int
	Search               query.StringFilter
	Category             string
	Action               string
	ResourceType         string
	ExcludeActions       []string
	ExcludeResourceTypes []string
	Username             query.StringFilter
	UserID               *uuid.UUID
	Status               string
	RiskLevel            string
	RequestPath          query.StringFilter
	CreatedAfter         *time.Time
	CreatedBefore        *time.Time
	SortBy               string
	SortOrder            string
}

type PlatformAuditListOptions struct {
	Page          int
	PageSize      int
	Search        query.StringFilter
	Category      string
	Action        string
	ResourceType  string
	Username      query.StringFilter
	UserID        *uuid.UUID
	Status        string
	RequestPath   query.StringFilter
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	SortBy        string
	SortOrder     string
}
