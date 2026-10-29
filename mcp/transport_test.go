package mcp_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/operations"
	"github.com/prulloac/weave/mcp"
)

func stdioServer(t *testing.T, token string) *mcp.Transport {
	t.Helper()
	executor := operations.New("test", engine.Options{})
	server, err := mcp.New(executor, mcp.Options{Bundle: fixture()})
	if err != nil {
		t.Fatal(err)
	}
	return &mcp.Transport{Server: server, Token: token, Version: "test"}
}

func initializeRequest(token string) string {
	params := map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "bdd", "version": "1"},
	}
	if token != "" {
		params["_meta"] = map[string]any{"token": token}
	}
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": params,
	})
	return string(data)
}

func requestLine(id int, method string, params any) string {
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": method, "params": params,
	})
	return string(data)
}

func stdioResponses(t *testing.T, transport *mcp.Transport, lines ...string) []map[string]any {
	t.Helper()
	var output bytes.Buffer
	err := transport.ServeStdio(context.Background(), strings.NewReader(strings.Join(lines, "\n")+"\n"), &output)
	if err != nil {
		t.Fatalf("stdio transport: %v", err)
	}
	return decodeLines(t, output.String())
}

func decodeLines(t *testing.T, output string) []map[string]any {
	t.Helper()
	responses := make([]map[string]any, 0)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var response map[string]any
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatalf("decode response %q: %v", line, err)
		}
		responses = append(responses, response)
	}
	return responses
}

func TestMCPStdioInitializeAndTools(t *testing.T) {
	transport := stdioServer(t, "secret")
	responses := stdioResponses(t, transport,
		initializeRequest("secret"),
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		requestLine(2, "tools/list", map[string]any{}),
	)
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d: %+v", len(responses), responses)
	}

	initResult, ok := responses[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("initialize response missing result: %+v", responses[0])
	}
	if initResult["protocolVersion"] != "2025-06-18" {
		t.Fatalf("unexpected protocol version: %+v", initResult)
	}
	if _, ok := initResult["capabilities"].(map[string]any)["tools"]; !ok {
		t.Fatalf("missing tools capability: %+v", initResult)
	}

	listResult, ok := responses[1]["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list response missing result: %+v", responses[1])
	}
	tools, ok := listResult["tools"].([]any)
	if !ok || len(tools) != 8 {
		t.Fatalf("expected 8 tools, got %+v", listResult)
	}
}

func TestMCPStdioBeforeInitialize(t *testing.T) {
	transport := stdioServer(t, "")
	responses := stdioResponses(t, transport, requestLine(1, "tools/list", map[string]any{}))
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if code := rpcErrorCode(responses[0]); code != -32002 {
		t.Fatalf("error code=%d, want -32002", code)
	}
}

func TestMCPStdioToolCall(t *testing.T) {
	transport := stdioServer(t, "")
	responses := stdioResponses(t, transport,
		initializeRequest(""),
		requestLine(2, "tools/call", map[string]any{
			"name":      "weave_search",
			"arguments": map[string]any{"query": "graph"},
		}),
		requestLine(3, "tools/call", map[string]any{
			"name":      "weave_show",
			"arguments": map[string]any{"id": "missing"},
		}),
	)
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}

	success, ok := responses[1]["result"].(map[string]any)
	if !ok || success["isError"] != false {
		t.Fatalf("expected successful tool result: %+v", responses[1])
	}
	if _, ok := success["structuredContent"].(map[string]any); !ok {
		t.Fatalf("missing structured content: %+v", success)
	}

	failure, ok := responses[2]["result"].(map[string]any)
	if !ok || failure["isError"] != true {
		t.Fatalf("expected tool error result: %+v", responses[2])
	}
	structured, _ := failure["structuredContent"].(map[string]any)
	if structured["code"] != string(contractv1.ErrConceptNotFound) {
		t.Fatalf("unexpected tool error: %+v", structured)
	}
	if _, hasProtocolError := responses[2]["error"]; hasProtocolError {
		t.Fatalf("tool errors must not be protocol errors: %+v", responses[2])
	}
}

func TestMCPStdioProtocolErrors(t *testing.T) {
	transport := stdioServer(t, "")
	responses := stdioResponses(t, transport,
		initializeRequest(""),
		`{"jsonrpc":"2.0","id":2,"method":`,
		requestLine(3, "weave_magic", map[string]any{}),
		requestLine(4, "ping", map[string]any{}),
	)
	if len(responses) != 4 {
		t.Fatalf("expected 4 responses, got %d: %+v", len(responses), responses)
	}
	if code := rpcErrorCode(responses[1]); code != -32700 {
		t.Fatalf("parse error code=%d, want -32700", code)
	}
	if code := rpcErrorCode(responses[2]); code != -32601 {
		t.Fatalf("method error code=%d, want -32601", code)
	}
	if _, ok := responses[3]["result"].(map[string]any); !ok {
		t.Fatalf("ping result missing: %+v", responses[3])
	}
}

func TestMCPStdioAuth(t *testing.T) {
	transport := stdioServer(t, "secret")

	missing := stdioResponses(t, transport, initializeRequest(""))
	if len(missing) != 1 {
		t.Fatalf("expected 1 response, got %d", len(missing))
	}
	if code := rpcErrorCode(missing[0]); code != -32001 {
		t.Fatalf("missing-token error code=%d, want -32001", code)
	}
	data, _ := missing[0]["error"].(map[string]any)["data"].(map[string]any)
	if data["code"] != "UNAUTHORIZED" {
		t.Fatalf("missing-token data=%+v", data)
	}

	wrong := stdioResponses(t, transport, initializeRequest("wrong"))
	if len(wrong) != 1 {
		t.Fatalf("expected 1 response, got %d", len(wrong))
	}
	if code := rpcErrorCode(wrong[0]); code != -32001 {
		t.Fatalf("wrong-token error code=%d, want -32001", code)
	}
	data, _ = wrong[0]["error"].(map[string]any)["data"].(map[string]any)
	if data["code"] != "UNAUTHORIZED" {
		t.Fatalf("wrong-token data=%+v", data)
	}
}

func TestMCPTCPTransport(t *testing.T) {
	transport := stdioServer(t, "secret")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go transport.Serve(ctx, listener)

	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(5 * time.Second))

	scanner := bufio.NewScanner(client)
	send := func(line string) map[string]any {
		if _, err := client.Write([]byte(line + "\n")); err != nil {
			t.Fatal(err)
		}
		if !scanner.Scan() {
			t.Fatalf("no response: %v", scanner.Err())
		}
		var response map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}

	if code := rpcErrorCode(send(requestLine(1, "tools/list", map[string]any{}))); code != -32002 {
		t.Fatalf("expected NOT_INITIALIZED, got %d", code)
	}
	if code := rpcErrorCode(send(initializeRequest("secret"))); code != 0 {
		t.Fatalf("initialize failed: %d", code)
	}
	list := send(requestLine(2, "tools/list", map[string]any{}))
	if _, ok := list["result"].(map[string]any)["tools"].([]any); !ok {
		t.Fatalf("tools/list failed: %+v", list)
	}
	call := send(requestLine(3, "tools/call", map[string]any{
		"name":      "weave_search",
		"arguments": map[string]any{"query": "graph"},
	}))
	if call["result"].(map[string]any)["isError"] != false {
		t.Fatalf("tool call failed: %+v", call)
	}
}

func rpcErrorCode(response map[string]any) int {
	failure, ok := response["error"].(map[string]any)
	if !ok {
		return 0
	}
	code, _ := failure["code"].(float64)
	return int(code)
}
