package search

import (
	"errors"
	"fmt"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// ErrInvalidFilter marks a filter that cannot be applied.
var ErrInvalidFilter = errors.New("invalid search filter")

// FilterMeta is the document metadata a filter matches against.
type FilterMeta struct {
	Type     string
	Status   string
	Tags     []string
	Keywords []string
}

// FilterSet is a validated, conjunctive set of metadata filters.
type FilterSet struct {
	filters []contractv1.SearchFilter
}

// NewFilterSet validates filters and returns a matcher. An empty slice yields an empty set.
func NewFilterSet(filters []contractv1.SearchFilter) (FilterSet, error) {
	for _, filter := range filters {
		switch filter.Field {
		case "type", "status", "tag", "keyword":
		default:
			return FilterSet{}, fmt.Errorf("%w: unknown field %q", ErrInvalidFilter, filter.Field)
		}
		if strings.TrimSpace(filter.Value) == "" {
			return FilterSet{}, fmt.Errorf("%w: empty value for %q", ErrInvalidFilter, filter.Field)
		}
	}
	return FilterSet{filters: append([]contractv1.SearchFilter(nil), filters...)}, nil
}

// Empty reports whether the set matches every document.
func (s FilterSet) Empty() bool { return len(s.filters) == 0 }

// Match reports whether meta satisfies every filter.
func (s FilterSet) Match(meta FilterMeta) bool {
	for _, filter := range s.filters {
		switch filter.Field {
		case "type":
			if meta.Type != filter.Value {
				return false
			}
		case "status":
			if meta.Status != filter.Value {
				return false
			}
		case "tag":
			if !containsExact(meta.Tags, filter.Value) {
				return false
			}
		case "keyword":
			if !containsFold(meta.Keywords, filter.Value) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func containsExact(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}
