package domain

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaxTagNameLength = 40

var (
	ErrInvalidTagName  = errors.New("tag name must contain 1 to 40 characters")
	ErrInvalidTagColor = errors.New("tag color must be a #RRGGBB hex color")
	ErrTagNameTaken    = errors.New("a tag with this name already exists")
)

var tagColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type Tag struct {
	ID    uuid.UUID `json:"id"`
	Name  TagName   `json:"name"`
	Color TagColor  `json:"color"`
}

type TagName string

func ParseTagName(value string) (TagName, error) {
	name := strings.TrimSpace(value)
	if name == "" || utf8.RuneCountInString(name) > MaxTagNameLength {
		return "", ErrInvalidTagName
	}
	return TagName(name), nil
}

type TagColor string

func ParseTagColor(value string) (TagColor, error) {
	if !tagColorPattern.MatchString(value) {
		return "", ErrInvalidTagColor
	}
	return TagColor(strings.ToUpper(value)), nil
}
