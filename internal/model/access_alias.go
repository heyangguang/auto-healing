package model

import accessmodel "github.com/company/auto-healing/internal/modules/access/model"

const (
	TenantStatusActive   = accessmodel.TenantStatusActive
	TenantStatusDisabled = accessmodel.TenantStatusDisabled

	InvitationStatusPending   = accessmodel.InvitationStatusPending
	InvitationStatusAccepted  = accessmodel.InvitationStatusAccepted
	InvitationStatusExpired   = accessmodel.InvitationStatusExpired
	InvitationStatusCancelled = accessmodel.InvitationStatusCancelled

	ImpersonationStatusPending   = accessmodel.ImpersonationStatusPending
	ImpersonationStatusApproved  = accessmodel.ImpersonationStatusApproved
	ImpersonationStatusRejected  = accessmodel.ImpersonationStatusRejected
	ImpersonationStatusActive    = accessmodel.ImpersonationStatusActive
	ImpersonationStatusCompleted = accessmodel.ImpersonationStatusCompleted
	ImpersonationStatusExpired   = accessmodel.ImpersonationStatusExpired
	ImpersonationStatusCancelled = accessmodel.ImpersonationStatusCancelled
)

var DefaultTenantID = accessmodel.DefaultTenantID

type User = accessmodel.User
type Role = accessmodel.Role
type Permission = accessmodel.Permission
type UserPlatformRole = accessmodel.UserPlatformRole
type RolePermission = accessmodel.RolePermission
type RefreshToken = accessmodel.RefreshToken
type TokenBlacklist = accessmodel.TokenBlacklist

type Tenant = accessmodel.Tenant
type UserTenantRole = accessmodel.UserTenantRole
type TenantInvitation = accessmodel.TenantInvitation

type ImpersonationRequest = accessmodel.ImpersonationRequest
type ImpersonationApprover = accessmodel.ImpersonationApprover
