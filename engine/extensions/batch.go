package extensions

import (
	"context"
	"encoding/json"
	"fmt"
)

const (
	// MaxBatchRequests bounds one batch envelope.
	MaxBatchRequests = 128
	// MaxBatchBytes bounds one batch request or response.
	MaxBatchBytes = 32 << 20
)

// BatchRequest is one opt-in batch envelope.
type BatchRequest struct {
	V        string            `json:"v"`
	Type     string            `json:"type"`
	Requests []json.RawMessage `json:"requests"`
}

// BatchResponse is one batch reply. Results stay in request order.
type BatchResponse struct {
	V       string            `json:"v"`
	Type    string            `json:"type"`
	Results []json.RawMessage `json:"results"`
}

type batchItemError struct {
	OK    *bool `json:"ok"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// EnrichBatch invokes a batch-capable enricher for up to MaxBatchRequests documents. The returned
// errors are indexed like the requests; an error for one item does not invalidate the others. A
// timeout, malformed envelope or wrong result count is returned as the third value and fails the
// whole batch.
func (r Runner) EnrichBatch(
	ctx context.Context,
	extension Extension,
	requests []EnrichmentRequest,
) ([]EnrichmentResponse, []error, error) {
	if len(requests) == 0 {
		return nil, nil, fmt.Errorf("%w: empty enrichment batch", ErrProtocol)
	}
	if len(requests) > MaxBatchRequests {
		return nil, nil, fmt.Errorf("%w: enrichment batch has %d requests, limit is %d", ErrProtocol, len(requests), MaxBatchRequests)
	}
	batch := BatchRequest{V: ProtocolVersion, Type: "batch_request", Requests: make([]json.RawMessage, 0, len(requests))}
	for _, request := range requests {
		request.V = ProtocolVersion
		request.Type = "enrichment_request"
		request.Body = truncate(request.Body, maxBody)
		data, err := json.Marshal(request)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %v", ErrProtocol, err)
		}
		batch.Requests = append(batch.Requests, data)
	}
	payload, err := json.Marshal(batch)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	if len(payload) > MaxBatchBytes {
		return nil, nil, fmt.Errorf("%w: %s batch request exceeds %d bytes", ErrProtocol, extension.Name, MaxBatchBytes)
	}
	var response BatchResponse
	if err := r.invokeVersionWithLimit(ctx, extension, batch, &response, MaxBatchBytes, ProtocolVersion); err != nil {
		return nil, nil, err
	}
	if response.Type != "batch_response" {
		return nil, nil, fmt.Errorf("%w: %s returned %q", ErrProtocol, extension.Name, response.Type)
	}
	if len(response.Results) != len(requests) {
		return nil, nil, fmt.Errorf("%w: %s returned %d results for %d requests", ErrProtocol, extension.Name, len(response.Results), len(requests))
	}
	responses := make([]EnrichmentResponse, len(requests))
	itemErrors := make([]error, len(requests))
	for index, result := range response.Results {
		var item batchItemError
		if err := json.Unmarshal(result, &item); err != nil {
			return nil, nil, fmt.Errorf("%w: invalid batch result %d", ErrProtocol, index)
		}
		if item.OK != nil && !*item.OK {
			code := "EXTENSION_FAILED"
			message := "extension item failed"
			if item.Error != nil {
				if item.Error.Code != "" {
					code = item.Error.Code
				}
				if item.Error.Message != "" {
					message = item.Error.Message
				}
			}
			itemErrors[index] = fmt.Errorf("%w: item %d: %s: %s", ErrFailed, index, code, message)
			continue
		}
		var enrichment EnrichmentResponse
		if err := json.Unmarshal(result, &enrichment); err != nil {
			return nil, nil, fmt.Errorf("%w: invalid enrichment result %d", ErrProtocol, index)
		}
		if enrichment.V != ProtocolVersion {
			return nil, nil, fmt.Errorf("%w: result %d has version %q", ErrProtocol, index, enrichment.V)
		}
		enrichment.InferredKeywords = normalizeKeywords(enrichment.InferredKeywords)
		responses[index] = enrichment
	}
	return responses, itemErrors, nil
}
