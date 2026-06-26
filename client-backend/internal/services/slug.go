package services

import (
	"regexp"
	"strings"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(input string) string {
	slug := strings.ToLower(strings.TrimSpace(input))
	slug = nonSlug.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "account"
	}
	return slug
}
