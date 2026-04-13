package httpapi

import (
	"time"

	accessmodel "github.com/company/auto-healing/internal/modules/access/model"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/company/auto-healing/internal/platform/modeltypes"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"github.com/google/uuid"
)

type commandBlacklistSearchOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type blacklistExemptionSearchOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type blacklistExemptionSearchField struct {
	Key     string                           `json:"key"`
	Label   string                           `json:"label"`
	Type    string                           `json:"type"`
	Options []blacklistExemptionSearchOption `json:"options,omitempty"`
}

type commandBlacklistSearchField struct {
	Key          string                         `json:"key"`
	Label        string                         `json:"label"`
	Type         string                         `json:"type"`
	SupportExact bool                           `json:"support_exact,omitempty"`
	Options      []commandBlacklistSearchOption `json:"options,omitempty"`
}

type commandBlacklistSearchSchemaResponse struct {
	Fields      []commandBlacklistSearchField `json:"fields"`
	GeneratedAt string                        `json:"generated_at"`
}

type commandBlacklistBatchToggleResponse struct {
	Count int64 `json:"count"`
}

type commandBlacklistSimulateResponse struct {
	Results      []opsservice.SimulateResult `json:"results"`
	TotalLines   int                         `json:"total_lines"`
	MatchCount   int                         `json:"match_count"`
	MatchedFiles map[string]int              `json:"matched_files"`
}

type auditLogResponse struct {
	ID                uuid.UUID         `json:"id"`
	UserID            *uuid.UUID        `json:"user_id,omitempty"`
	Username          string            `json:"username,omitempty"`
	PrincipalUsername string            `json:"principal_username,omitempty"`
	SubjectScope      string            `json:"subject_scope,omitempty"`
	SubjectTenantID   *uuid.UUID        `json:"subject_tenant_id,omitempty"`
	SubjectTenantName string            `json:"subject_tenant_name,omitempty"`
	FailureReason     string            `json:"failure_reason,omitempty"`
	AuthMethod        string            `json:"auth_method,omitempty"`
	IPAddress         string            `json:"ip_address,omitempty"`
	UserAgent         string            `json:"user_agent,omitempty"`
	Category          string            `json:"category"`
	Action            string            `json:"action"`
	ResourceType      string            `json:"resource_type"`
	ResourceID        *uuid.UUID        `json:"resource_id,omitempty"`
	ResourceName      string            `json:"resource_name,omitempty"`
	RequestMethod     string            `json:"request_method,omitempty"`
	RequestPath       string            `json:"request_path,omitempty"`
	RequestBody       modeltypes.JSON   `json:"request_body,omitempty"`
	ResponseStatus    *int              `json:"response_status,omitempty"`
	Changes           modeltypes.JSON   `json:"changes,omitempty"`
	Status            string            `json:"status"`
	ErrorMessage      string            `json:"error_message,omitempty"`
	RiskLevel         string            `json:"risk_level"`
	RiskReason        string            `json:"risk_reason"`
	CreatedAt         time.Time         `json:"created_at"`
	User              *accessmodel.User `json:"user,omitempty"`
}

type platformAuditLogResponse struct {
	ID                uuid.UUID       `json:"id"`
	UserID            *uuid.UUID      `json:"user_id,omitempty"`
	Username          string          `json:"username,omitempty"`
	PrincipalUsername string          `json:"principal_username,omitempty"`
	SubjectScope      string          `json:"subject_scope,omitempty"`
	SubjectTenantID   *uuid.UUID      `json:"subject_tenant_id,omitempty"`
	SubjectTenantName string          `json:"subject_tenant_name,omitempty"`
	FailureReason     string          `json:"failure_reason,omitempty"`
	AuthMethod        string          `json:"auth_method,omitempty"`
	IPAddress         string          `json:"ip_address,omitempty"`
	UserAgent         string          `json:"user_agent,omitempty"`
	Category          string          `json:"category"`
	Action            string          `json:"action"`
	ResourceType      string          `json:"resource_type"`
	ResourceID        *uuid.UUID      `json:"resource_id,omitempty"`
	ResourceName      string          `json:"resource_name,omitempty"`
	RequestMethod     string          `json:"request_method,omitempty"`
	RequestPath       string          `json:"request_path,omitempty"`
	RequestBody       modeltypes.JSON `json:"request_body,omitempty"`
	ResponseStatus    *int            `json:"response_status,omitempty"`
	Changes           modeltypes.JSON `json:"changes,omitempty"`
	Status            string          `json:"status"`
	ErrorMessage      string          `json:"error_message,omitempty"`
	RiskLevel         string          `json:"risk_level"`
	RiskReason        string          `json:"risk_reason"`
	CreatedAt         time.Time       `json:"created_at"`
}

type highRiskAuditLogResponse struct {
	ID           uuid.UUID         `json:"id"`
	Username     string            `json:"username,omitempty"`
	Action       string            `json:"action"`
	ResourceType string            `json:"resource_type"`
	ResourceName string            `json:"resource_name,omitempty"`
	Status       string            `json:"status"`
	IPAddress    string            `json:"ip_address,omitempty"`
	RiskReason   string            `json:"risk_reason"`
	CreatedAt    time.Time         `json:"created_at"`
	User         *accessmodel.User `json:"user,omitempty"`
}

type platformHighRiskAuditLogResponse struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username,omitempty"`
	Category     string    `json:"category"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceName string    `json:"resource_name,omitempty"`
	Status       string    `json:"status"`
	IPAddress    string    `json:"ip_address,omitempty"`
	RiskReason   string    `json:"risk_reason"`
	CreatedAt    time.Time `json:"created_at"`
}

func newAuditLogResponse(log platformmodel.AuditLog) auditLogResponse {
	riskLevel, riskReason := auditRiskFields(log.Action, log.ResourceType)
	return auditLogResponse{
		ID:                log.ID,
		UserID:            log.UserID,
		Username:          log.Username,
		PrincipalUsername: log.Username,
		IPAddress:         log.IPAddress,
		UserAgent:         log.UserAgent,
		Category:          log.Category,
		Action:            log.Action,
		ResourceType:      log.ResourceType,
		ResourceID:        log.ResourceID,
		ResourceName:      log.ResourceName,
		RequestMethod:     log.RequestMethod,
		RequestPath:       log.RequestPath,
		RequestBody:       sanitizeAuditPayload(log.RequestBody),
		ResponseStatus:    log.ResponseStatus,
		Changes:           sanitizeAuditPayload(log.Changes),
		Status:            log.Status,
		ErrorMessage:      log.ErrorMessage,
		RiskLevel:         riskLevel,
		RiskReason:        riskReason,
		CreatedAt:         log.CreatedAt,
		User:              log.User,
	}
}

func newAuditLogResponseFromPlatformLog(log platformmodel.PlatformAuditLog) auditLogResponse {
	riskLevel := auditrepo.GetRiskLevel(log.Action, log.ResourceType)
	riskReason := auditrepo.GetRiskReason(log.Action, log.ResourceType)
	return auditLogResponse{
		ID:                log.ID,
		UserID:            log.UserID,
		Username:          log.Username,
		PrincipalUsername: log.PrincipalUsername,
		SubjectScope:      log.SubjectScope,
		SubjectTenantID:   log.SubjectTenantID,
		SubjectTenantName: log.SubjectTenantName,
		FailureReason:     log.FailureReason,
		AuthMethod:        log.AuthMethod,
		IPAddress:         log.IPAddress,
		UserAgent:         log.UserAgent,
		Category:          log.Category,
		Action:            log.Action,
		ResourceType:      log.ResourceType,
		ResourceID:        log.ResourceID,
		ResourceName:      log.ResourceName,
		RequestMethod:     log.RequestMethod,
		RequestPath:       log.RequestPath,
		RequestBody:       sanitizeAuditPayload(log.RequestBody),
		ResponseStatus:    log.ResponseStatus,
		Changes:           sanitizeAuditPayload(log.Changes),
		Status:            log.Status,
		ErrorMessage:      log.ErrorMessage,
		RiskLevel:         riskLevel,
		RiskReason:        riskReason,
		CreatedAt:         log.CreatedAt,
	}
}

func newPlatformAuditLogResponse(log platformmodel.PlatformAuditLog) platformAuditLogResponse {
	return platformAuditLogResponse{
		ID:                log.ID,
		UserID:            log.UserID,
		Username:          log.Username,
		PrincipalUsername: log.PrincipalUsername,
		SubjectScope:      log.SubjectScope,
		SubjectTenantID:   log.SubjectTenantID,
		SubjectTenantName: log.SubjectTenantName,
		FailureReason:     log.FailureReason,
		AuthMethod:        log.AuthMethod,
		IPAddress:         log.IPAddress,
		UserAgent:         log.UserAgent,
		Category:          log.Category,
		Action:            log.Action,
		ResourceType:      log.ResourceType,
		ResourceID:        log.ResourceID,
		ResourceName:      log.ResourceName,
		RequestMethod:     log.RequestMethod,
		RequestPath:       log.RequestPath,
		RequestBody:       sanitizeAuditPayload(log.RequestBody),
		ResponseStatus:    log.ResponseStatus,
		Changes:           sanitizeAuditPayload(log.Changes),
		Status:            log.Status,
		ErrorMessage:      log.ErrorMessage,
		RiskLevel:         auditrepo.GetRiskLevel(log.Action, log.ResourceType),
		RiskReason:        auditrepo.GetRiskReason(log.Action, log.ResourceType),
		CreatedAt:         log.CreatedAt,
	}
}

func newHighRiskAuditLogResponse(log platformmodel.AuditLog) highRiskAuditLogResponse {
	_, riskReason := auditRiskFields(log.Action, log.ResourceType)
	return highRiskAuditLogResponse{
		ID:           log.ID,
		Username:     log.Username,
		Action:       log.Action,
		ResourceType: log.ResourceType,
		ResourceName: log.ResourceName,
		Status:       log.Status,
		IPAddress:    log.IPAddress,
		RiskReason:   riskReason,
		CreatedAt:    log.CreatedAt,
		User:         log.User,
	}
}

func newPlatformHighRiskAuditLogResponse(log platformmodel.PlatformAuditLog) platformHighRiskAuditLogResponse {
	return platformHighRiskAuditLogResponse{
		ID:           log.ID,
		Username:     log.Username,
		Category:     log.Category,
		Action:       log.Action,
		ResourceType: log.ResourceType,
		ResourceName: log.ResourceName,
		Status:       log.Status,
		IPAddress:    log.IPAddress,
		RiskReason:   auditrepo.GetRiskReason(log.Action, log.ResourceType),
		CreatedAt:    log.CreatedAt,
	}
}
