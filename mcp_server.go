package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type MCPServer struct {
	client *futabusClient
}

type MCPRequest struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewMCPServer() *MCPServer {
	return &MCPServer{
		client: NewFutabusClient(),
	}
}

func (s *MCPServer) handleRequest(req MCPRequest) MCPResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

func (s *MCPServer) handleInitialize(req MCPRequest) MCPResponse {
	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]string{
				"name":    "mcp-futabus",
				"version": "1.0.0",
			},
		},
	}
}

func (s *MCPServer) handleToolsList(req MCPRequest) MCPResponse {
	tools := []map[string]interface{}{
		{
			"name":        "search_pickup_points",
			"description": "Search for pickup point groups and areas by keyword (city or province name)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"keyword": map[string]string{
						"type":        "string",
						"description": "Search keyword, e.g. city or province name",
					},
				},
				"required": []string{"keyword"},
			},
		},
		{
			"name":        "search_routes",
			"description": "Search available routes between two areas on a given date. Use search_pickup_points first to get area IDs.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"originAreaId": map[string]string{
						"type":        "string",
						"description": "Area ID of the origin (from search_pickup_points)",
					},
					"destAreaId": map[string]string{
						"type":        "string",
						"description": "Area ID of the destination (from search_pickup_points)",
					},
					"fromDate": map[string]string{
						"type":        "string",
						"description": "Travel date in format YYYY-MM-DD",
					},
				},
				"required": []string{"originAreaId", "destAreaId", "fromDate"},
			},
		},
		{
			"name":        "search_trips",
			"description": "Search for available trips by route IDs and date range. Use search_routes first to get route IDs.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"routeIds": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "List of route IDs (from search_routes)",
					},
					"fromDate": map[string]string{
						"type":        "string",
						"description": "Start date in format YYYY-MM-DD",
					},
					"toDate": map[string]string{
						"type":        "string",
						"description": "End date in format YYYY-MM-DD",
					},
				},
				"required": []string{"routeIds", "fromDate", "toDate"},
			},
		},
		{
			"name":        "get_seat_diagram",
			"description": "Get the seat diagram for a specific trip. Use search_trips first to get a trip ID.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tripId": map[string]string{
						"type":        "string",
						"description": "Trip ID (from search_trips)",
					},
				},
				"required": []string{"tripId"},
			},
		},
		{
			"name":        "get_departments_in_way",
			"description": "Get all pickup/dropoff stops for a way (direction) of a route. Use search_trips to get wayId and routeId.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"wayId": map[string]string{
						"type":        "string",
						"description": "Way ID (from search_trips)",
					},
					"routeId": map[string]string{
						"type":        "string",
						"description": "Route ID (from search_trips or search_routes)",
					},
				},
				"required": []string{"wayId", "routeId"},
			},
		},
	}

	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

func (s *MCPServer) handleToolsCall(req MCPRequest) MCPResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32602, Message: "Invalid params"},
		}
	}

	ctx := context.Background()

	switch params.Name {
	case "search_pickup_points":
		return s.callSearchPickupPoints(ctx, req.ID, params.Arguments)
	case "search_routes":
		return s.callSearchRoutes(ctx, req.ID, params.Arguments)
	case "search_trips":
		return s.callSearchTrips(ctx, req.ID, params.Arguments)
	case "get_seat_diagram":
		return s.callGetSeatDiagram(ctx, req.ID, params.Arguments)
	case "get_departments_in_way":
		return s.callGetDepartmentsInWay(ctx, req.ID, params.Arguments)
	default:
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32601, Message: "Tool not found"},
		}
	}
}

// normalizeDate converts a date string in YYYY-MM-DD format to RFC3339 format
// (YYYY-MM-DDT00:00:00Z). If the string already contains 'T', it is returned as-is.
func normalizeDate(date string) string {
	if len(date) == 10 {
		return date + "T00:00:00Z"
	}
	return date
}

func textResult(id interface{}, text string) MCPResponse {
	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": text},
			},
		},
	}
}

func errorResult(id interface{}, err error) MCPResponse {
	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Error:   &MCPError{Code: -32000, Message: err.Error()},
	}
}

func invalidArgs(id interface{}) MCPResponse {
	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Error:   &MCPError{Code: -32602, Message: "Invalid arguments"},
	}
}

func (s *MCPServer) callSearchPickupPoints(ctx context.Context, id interface{}, args json.RawMessage) MCPResponse {
	var params struct {
		Keyword string `json:"keyword"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return invalidArgs(id)
	}

	groups, areas, err := s.client.SearchPickupPoints(ctx, params.Keyword)
	if err != nil {
		return errorResult(id, err)
	}

	result := map[string]interface{}{
		"pickupPointGroups": groups,
		"areas":             areas,
	}
	data, err := json.Marshal(result)
	if err != nil {
		return errorResult(id, err)
	}
	return textResult(id, string(data))
}

func (s *MCPServer) callSearchRoutes(ctx context.Context, id interface{}, args json.RawMessage) MCPResponse {
	var params struct {
		OriginAreaID string `json:"originAreaId"`
		DestAreaID   string `json:"destAreaId"`
		FromDate     string `json:"fromDate"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return invalidArgs(id)
	}

	routes, err := s.client.SearchRoutes(ctx, params.OriginAreaID, params.DestAreaID, normalizeDate(params.FromDate))
	if err != nil {
		return errorResult(id, err)
	}

	data, err := json.Marshal(routes)
	if err != nil {
		return errorResult(id, err)
	}
	return textResult(id, string(data))
}

func (s *MCPServer) callSearchTrips(ctx context.Context, id interface{}, args json.RawMessage) MCPResponse {
	var params struct {
		RouteIDs []string `json:"routeIds"`
		FromDate string   `json:"fromDate"`
		ToDate   string   `json:"toDate"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return invalidArgs(id)
	}

	trips, err := s.client.SearchTripsByRoute(ctx, params.RouteIDs, normalizeDate(params.FromDate), normalizeDate(params.ToDate))
	if err != nil {
		return errorResult(id, err)
	}

	data, err := json.Marshal(trips)
	if err != nil {
		return errorResult(id, err)
	}
	return textResult(id, string(data))
}

func (s *MCPServer) callGetSeatDiagram(ctx context.Context, id interface{}, args json.RawMessage) MCPResponse {
	var params struct {
		TripID string `json:"tripId"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return invalidArgs(id)
	}

	seats, err := s.client.GetSeatDiagram(ctx, params.TripID)
	if err != nil {
		return errorResult(id, err)
	}

	data, err := json.Marshal(seats)
	if err != nil {
		return errorResult(id, err)
	}
	return textResult(id, string(data))
}

func (s *MCPServer) callGetDepartmentsInWay(ctx context.Context, id interface{}, args json.RawMessage) MCPResponse {
	var params struct {
		WayID   string `json:"wayId"`
		RouteID string `json:"routeId"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return invalidArgs(id)
	}

	depts, err := s.client.GetDepartmentsInWay(ctx, params.WayID, params.RouteID)
	if err != nil {
		return errorResult(id, err)
	}

	data, err := json.Marshal(depts)
	if err != nil {
		return errorResult(id, err)
	}
	return textResult(id, string(data))
}

func (s *MCPServer) Run() error {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var req MCPRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("error decoding request: %v", err)
		}

		resp := s.handleRequest(req)
		if err := encoder.Encode(resp); err != nil {
			return fmt.Errorf("error encoding response: %v", err)
		}
	}
}

// RunHTTP starts an HTTP Streamable MCP server on the given address.
// Clients send JSON-RPC requests via POST /mcp.
// If the client sends Accept: text/event-stream, responses are wrapped in SSE.
// GET /mcp keeps an SSE connection open for server-initiated messages.
func (s *MCPServer) RunHTTP(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.mcpHTTPHandler)
	fmt.Fprintf(os.Stderr, "HTTP Streamable MCP listening on %s/mcp\n", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *MCPServer) mcpHTTPHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")

	switch r.Method {
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)

	case http.MethodPost:
		var req MCPRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		resp := s.handleRequest(req)

		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			data, _ := json.Marshal(resp)
			fmt.Fprintf(w, "data: %s\n\n", data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}

	case http.MethodGet:
		// SSE stream for server-initiated messages; keep open until client disconnects.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done()

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
