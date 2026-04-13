package audit

const (
	authCategoryStored = "auth"
	authCategoryLegacy = "login"
)

func isAuthCategoryFilter(category string) bool {
	return category == authCategoryStored || category == authCategoryLegacy
}

func isLegacyLoginCategory(category string) bool {
	return category == authCategoryLegacy
}

func IsAuthCategoryFilter(category string) bool {
	return isAuthCategoryFilter(category)
}
