package mcp

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
)

const defaultMaxFrameBytes = 1 << 20

// Transport serves an MCP Server over stdio or TCP.
type Transport struct {
	Server        *Server
	Token         string
	MaxFrameBytes int
	Version       string
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type sessionState struct {
	initialized bool
}

// ServeStdio serves newline-delimited JSON-RPC over stdin/stdout.
func (t *Transport) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), t.maxFrame())
	state := sessionState{}
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		response, closeSession := t.handleLine(scanner.Bytes(), &state)
		if response != nil {
			if err := writeResponse(out, response); err != nil {
				return err
			}
		}
		if closeSession {
			return nil
		}
	}
	return scanner.Err()
}

// Serve accepts TCP connections until the listener or context closes.
func (t *Transport) Serve(ctx context.Context, listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go func() {
			_ = t.ServeConn(ctx, conn)
		}()
	}
}

// ServeConn serves one TCP connection.
func (t *Transport) ServeConn(ctx context.Context, conn net.Conn) error {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), t.maxFrame())
	state := sessionState{}
	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil
		}
		response, closeSession := t.handleLine(scanner.Bytes(), &state)
		if response != nil {
			if err := writeResponse(conn, response); err != nil {
				return err
			}
		}
		if closeSession {
			return nil
		}
	}
	return scanner.Err()
}

func (t *Transport) handleLine(line []byte, state *sessionState) (map[string]any, bool) {
	var request rpcRequest
	if err := json.Unmarshal(line, &request); err != nil {
		return errorResponse(nil, -32700, "parse error", nil), false
	}
	if request.Method == "" {
		return errorResponse(request.ID, -32600, "invalid request", nil), false
	}

	switch request.Method {
	case "initialize":
		if t.Token != "" {
			token := initializeToken(request.Params)
			if subtle.ConstantTimeCompare([]byte(token), []byte(t.Token)) != 1 {
				return errorResponse(request.ID, -32001, "unauthorized", map[string]any{
					"code": "UNAUTHORIZED",
				}), true
			}
		}
		state.initialized = true
		version := t.Version
		if version == "" {
			version = "dev"
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"result": map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities": map[string]any{
					"tools":     map[string]any{},
					"resources": map[string]any{},
				},
				"serverInfo": map[string]any{"name": "weave", "version": version},
			},
		}, false
	case "notifications/initialized":
		state.initialized = true
		return nil, false
	case "ping":
		return map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": map[string]any{}}, false
	case "tools/list":
		if !state.initialized {
			return errorResponse(request.ID, -32002, "not initialized", map[string]any{
				"code": "NOT_INITIALIZED",
			}), false
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"result":  map[string]any{"tools": t.Server.Tools()},
		}, false
	case "tools/call":
		if !state.initialized {
			return errorResponse(request.ID, -32002, "not initialized", map[string]any{
				"code": "NOT_INITIALIZED",
			}), false
		}
		return t.handleToolCall(request), false
	case "resources/list":
		if !state.initialized {
			return errorResponse(request.ID, -32002, "not initialized", map[string]any{
				"code": "NOT_INITIALIZED",
			}), false
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"result":  map[string]any{"resources": t.Server.Resources()},
		}, false
	case "resources/templates/list":
		if !state.initialized {
			return errorResponse(request.ID, -32002, "not initialized", map[string]any{
				"code": "NOT_INITIALIZED",
			}), false
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"result":  map[string]any{"resourceTemplates": t.Server.ResourceTemplates()},
		}, false
	case "resources/read":
		if !state.initialized {
			return errorResponse(request.ID, -32002, "not initialized", map[string]any{
				"code": "NOT_INITIALIZED",
			}), false
		}
		return t.handleResourceRead(request), false
	default:
		return errorResponse(request.ID, -32601, "method not found", nil), false
	}
}

func (t *Transport) handleResourceRead(request rpcRequest) map[string]any {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil || params.URI == "" {
		return errorResponse(request.ID, -32602, "invalid params", nil)
	}
	content, err := t.Server.ReadResource(context.Background(), params.URI)
	if err != nil {
		return errorResponse(request.ID, -32602, err.Error(), map[string]any{
			"code": "RESOURCE_NOT_FOUND",
		})
	}
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      request.ID,
		"result":  map[string]any{"contents": []any{content}},
	}
}

func (t *Transport) handleToolCall(request rpcRequest) map[string]any {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil || params.Name == "" {
		return errorResponse(request.ID, -32602, "invalid params", nil)
	}
	result, err := t.Server.Call(context.Background(), params.Name, params.Arguments)
	if err != nil {
		message := err.Error()
		code := "INTERNAL"
		if failure, ok := err.(*Error); ok {
			code = string(failure.Code)
			message = failure.Message
		}
		structured := map[string]any{"code": code, "message": message}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"result": map[string]any{
				"content": []any{map[string]any{
					"type": "text",
					"text": message,
				}},
				"structuredContent": structured,
				"isError":           true,
			},
		}
	}
	data, _ := json.Marshal(result)
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      request.ID,
		"result": map[string]any{
			"content": []any{map[string]any{
				"type": "text",
				"text": string(data),
			}},
			"structuredContent": result,
			"isError":           false,
		},
	}
}

func (t *Transport) maxFrame() int {
	if t.MaxFrameBytes > 0 {
		return t.MaxFrameBytes
	}
	return defaultMaxFrameBytes
}

func initializeToken(raw json.RawMessage) string {
	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil {
		return ""
	}
	meta, _ := params["_meta"].(map[string]any)
	token, _ := meta["token"].(string)
	return strings.TrimSpace(token)
}

func errorResponse(id any, code int, message string, data any) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": rpcError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

func writeResponse(writer io.Writer, response map[string]any) error {
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "%s\n", data); err != nil {
		return err
	}
	return nil
}
