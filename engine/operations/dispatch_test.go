package operations

import "testing"

func TestEveryOperationHasAHandler(t *testing.T) {
	for _, descriptor := range operationDescriptors {
		_, preflight := preflightHandlers[descriptor.Op]
		_, request := requestHandlers[descriptor.Op]
		_, engine := engineHandlers[descriptor.Op]
		if !preflight && !request && !engine {
			t.Errorf("operation %q has no handler", descriptor.Op)
		}
	}
}

func TestNoOrphanHandlers(t *testing.T) {
	known := make(map[string]bool, len(operationDescriptors))
	for _, descriptor := range operationDescriptors {
		known[string(descriptor.Op)] = true
	}
	for op := range preflightHandlers {
		if !known[string(op)] {
			t.Errorf("preflight handler for unknown operation %q", op)
		}
	}
	for op := range requestHandlers {
		if !known[string(op)] {
			t.Errorf("request handler for unknown operation %q", op)
		}
	}
	for op := range engineHandlers {
		if !known[string(op)] {
			t.Errorf("engine handler for unknown operation %q", op)
		}
	}
}
