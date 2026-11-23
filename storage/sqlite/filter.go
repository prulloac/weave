package sqlite

import (
	"fmt"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// filterSQL renders metadata filters as parameterized predicates on the `documents` alias `d`.
func filterSQL(filters []contractv1.SearchFilter) (string, []any, error) {
	var where strings.Builder
	args := make([]any, 0, len(filters))
	for _, filter := range filters {
		switch filter.Field {
		case "type":
			where.WriteString(" AND d.doc_type = ?")
			args = append(args, filter.Value)
		case "status":
			where.WriteString(" AND d.status = ?")
			args = append(args, filter.Value)
		case "tag":
			where.WriteString(" AND EXISTS (SELECT 1 FROM json_each(d.tags_json) WHERE value = ?)")
			args = append(args, filter.Value)
		case "keyword":
			where.WriteString(" AND EXISTS (SELECT 1 FROM json_each(d.keywords_json) WHERE lower(value) = lower(?))")
			args = append(args, filter.Value)
		default:
			return "", nil, fmt.Errorf("unknown search filter field %q", filter.Field)
		}
	}
	return where.String(), args, nil
}
