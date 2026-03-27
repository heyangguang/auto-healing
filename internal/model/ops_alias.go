package model

import opsmodel "github.com/company/auto-healing/internal/modules/ops/model"

const (
	SettingTypeInt    = opsmodel.SettingTypeInt
	SettingTypeString = opsmodel.SettingTypeString
	SettingTypeBool   = opsmodel.SettingTypeBool
	SettingTypeJSON   = opsmodel.SettingTypeJSON
)

type BlacklistExemption = opsmodel.BlacklistExemption
type CommandBlacklist = opsmodel.CommandBlacklist
type CommandBlacklistViolation = opsmodel.CommandBlacklistViolation
type Dictionary = opsmodel.Dictionary
type PlatformSetting = opsmodel.PlatformSetting
type TenantBlacklistOverride = opsmodel.TenantBlacklistOverride
