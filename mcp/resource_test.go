package mcp_test

import (
	"encoding/json"
	"slices"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func TestMCPResourceDiscovery(t *testing.T) {
	transport := stdioServer(t, "")
	responses := stdioResponses(t, transport,
		initializeRequest(""),
		requestLine(2, "resources/list", map[string]any{}),
		requestLine(3, "resources/templates/list", map[string]any{}),
	)
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}

	capabilities := responses[0]["result"].(map[string]any)["capabilities"].(map[string]any)
	if _, ok := capabilities["resources"]; !ok {
		t.Fatalf("missing resources capability: %+v", capabilities)
	}

	list := responses[1]["result"].(map[string]any)["resources"].([]any)
	uris := make([]string, 0, len(list))
	for _, entry := range list {
		resource := entry.(map[string]any)
		uris = append(uris, resource["uri"].(string))
	}
	for _, want := range []string{"weave://bundle/index", "weave://bundle/stats"} {
		if !slices.Contains(uris, want) {
			t.Fatalf("missing resource %s: %v", want, uris)
		}
	}

	templates := responses[2]["result"].(map[string]any)["resourceTemplates"].([]any)
	templateURIs := make([]string, 0, len(templates))
	for _, entry := range templates {
		template := entry.(map[string]any)
		templateURIs = append(templateURIs, template["uriTemplate"].(string))
	}
	for _, want := range []string{
		"weave://concept/{id}",
		"weave://graph/neighborhood/{id}",
	} {
		if !slices.Contains(templateURIs, want) {
			t.Fatalf("missing template %s: %v", want, templateURIs)
		}
	}
}

func TestMCPResourceReads(t *testing.T) {
	transport := stdioServer(t, "")
	responses := stdioResponses(t, transport,
		initializeRequest(""),
		readResourceLine(2, "weave://bundle/index"),
		readResourceLine(3, "weave://bundle/stats"),
		readResourceLine(4, "weave://concept/concepts/okf-bundle"),
		readResourceLine(5, "weave://graph/neighborhood/concepts/okf-bundle"),
	)
	if len(responses) != 5 {
		t.Fatalf("expected 5 responses, got %d", len(responses))
	}

	index := resourcePayload[[]contractv1.IndexEntry](t, responses[1])
	if len(index) == 0 {
		t.Fatal("expected index entries")
	}

	stats := resourcePayload[map[string]any](t, responses[2])
	if stats["concepts"].(float64) != 5 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if _, ok := stats["edges"]; !ok {
		t.Fatalf("stats missing edges: %+v", stats)
	}
	if _, ok := stats["orphans"]; !ok {
		t.Fatalf("stats missing orphans: %+v", stats)
	}
	if _, ok := stats["valid"]; !ok {
		t.Fatalf("stats missing valid: %+v", stats)
	}

	show := resourcePayload[contractv1.ShowResult](t, responses[3])
	if show.ID != "concepts/okf-bundle" {
		t.Fatalf("unexpected concept: %+v", show)
	}

	neighborhood := resourcePayload[contractv1.RelateResult](t, responses[4])
	if neighborhood.Root != "concepts/okf-bundle" {
		t.Fatalf("unexpected neighborhood: %+v", neighborhood)
	}
}

func TestMCPResourceErrors(t *testing.T) {
	transport := stdioServer(t, "")
	responses := stdioResponses(t, transport,
		readResourceLine(1, "weave://bundle/index"),
		initializeRequest(""),
		readResourceLine(2, "weave://bundle/missing"),
	)
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}
	if code := rpcErrorCode(responses[0]); code != -32002 {
		t.Fatalf("before initialize code=%d, want -32002", code)
	}
	if code := rpcErrorCode(responses[2]); code != -32602 {
		t.Fatalf("unknown resource code=%d, want -32602", code)
	}
	data, _ := responses[2]["error"].(map[string]any)["data"].(map[string]any)
	if data["code"] != "RESOURCE_NOT_FOUND" {
		t.Fatalf("unexpected error data: %+v", data)
	}
}

func readResourceLine(id int, uri string) string {
	return requestLine(id, "resources/read", map[string]any{"uri": uri})
}

func resourcePayload[T any](t *testing.T, response map[string]any) T {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %+v", response)
	}
	contents, ok := result["contents"].([]any)
	if !ok || len(contents) != 1 {
		t.Fatalf("unexpected contents: %+v", result)
	}
	content := contents[0].(map[string]any)
	if content["mimeType"] != "application/json" {
		t.Fatalf("unexpected mime type: %+v", content)
	}
	var payload T
	if err := json.Unmarshal([]byte(content["text"].(string)), &payload); err != nil {
		t.Fatalf("decode resource payload: %v", err)
	}
	return payload
}
