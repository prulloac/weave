package operations

import (
	"errors"
	"fmt"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/typeschema"
)

// typeSchemaFailure loads and applies .weave/types schemas. It returns a failure code and message
// when the write must stop; permissive violations are emitted as warnings and do not stop it.
func (e *Executor) typeSchemaFailure(root string, bundle *contractv1.Bundle, permissive bool) (contractv1.ErrorCode, string, bool) {
	set, err := typeschema.Load(root)
	if err != nil {
		if errors.Is(err, typeschema.ErrUnsupported) {
			return contractv1.ErrTypeSchemaUnsupported, err.Error(), true
		}
		return contractv1.ErrTypeSchemaInvalid, err.Error(), true
	}
	if set.Empty() || bundle == nil {
		return "", "", false
	}
	var violations []string
	for _, concept := range bundle.Concepts {
		for _, violation := range set.Validate(concept) {
			violations = append(violations, fmt.Sprintf("%s: %s", violation.Path, violation.Message))
		}
	}
	if len(violations) == 0 {
		return "", "", false
	}
	if permissive {
		for _, violation := range violations {
			e.warn("type schema: %s", violation)
		}
		return "", "", false
	}
	message := "type schema violations: " + strings.Join(violations, "; ")
	return contractv1.ErrTypeSchemaInvalid, message, true
}
