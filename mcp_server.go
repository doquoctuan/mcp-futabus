package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
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
			"name":        "get_origin_codes",
			"description": "Get all available origin codes for bus routes",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "get_routes",
			"description": "Get routes between origin and destination",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"originCode": map[string]string{"type": "string", "description": "Origin code"},
					"destCode":   map[string]string{"type": "string", "description": "Destination code"},
					"fromId":     map[string]string{"type": "integer", "description": "Id of origin office"},
					"toId":       map[string]string{"type": "integer", "description": "Id of destination office"},
				},
				"required": []string{"originCode", "destCode", "fromId", "toId"},
			},
		},
		{
			"name":        "search_trips",
			"description": "Search for available trips",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"routeIds": map[string]interface{}{"type": "array", "items": map[string]string{"type": "integer"}},
					"fromDate": map[string]string{
						"type":        "string",
						"description": "Start date and time in format YYYY-MM-DD HH:MM (e.g., 2024-01-01 08:00)",
					},
					"toDate": map[string]string{
						"type":        "string",
						"description": "End date and time in format YYYY-MM-DD HH:MM (e.g., 2024-01-02 20:00)",
					},
					"ticketCount": map[string]string{"type": "integer", "description": "Number of tickets"},
				},
				"required": []string{"routeIds", "fromDate", "toDate", "ticketCount"},
			},
		},
		{
			"name":        "get_price_list",
			"description": "Get minimum prices for routes within a date range",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"routeIds": map[string]interface{}{"type": "array", "items": map[string]string{"type": "integer"}},
					"fromDate": map[string]string{"type": "string", "description": "Start date (MM-DD-YYYY)"},
					"toDate":   map[string]string{"type": "string", "description": "End date (MM-DD-YYYY)"},
				},
				"required": []string{"routeIds", "fromDate", "toDate"},
			},
		},
		{
			"name":        "get_office_group",
			"description": "Get office groups for specific routes",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"routeIds": map[string]interface{}{"type": "array", "items": map[string]string{"type": "integer"}},
				},
				"required": []string{"routeIds"},
			},
		},
		{
			"name":        "get_booking_stops",
			"description": "Get booking stops for a specific route",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"routeId": map[string]string{"type": "integer", "description": "Route ID"},
					"wayId":   map[string]string{"type": "integer", "description": "Way ID"},
				},
				"required": []string{"routeId", "wayId"},
			},
		},
		{
			"name":        "get_booking_seats",
			"description": "Get available seats for a specific trip",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"routeId":       map[string]string{"type": "integer", "description": "Route ID"},
					"tripId":        map[string]string{"type": "integer", "description": "Trip ID"},
					"departureDate": map[string]string{"type": "string", "description": "Departure date"},
					"departureTime": map[string]string{"type": "string", "description": "Departure time"},
					"kind":          map[string]string{"type": "string", "description": "Kind of booking"},
				},
				"required": []string{"routeId", "tripId", "departureDate", "departureTime", "kind"},
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

	switch params.Name {
	case "get_origin_codes":
		return s.callGetOriginCodes(req.ID)
	case "get_routes":
		return s.callGetRoutes(req.ID, params.Arguments)
	case "search_trips":
		return s.callSearchTrips(req.ID, params.Arguments)
	case "get_price_list":
		return s.callGetPriceList(req.ID, params.Arguments)
	case "get_office_group":
		return s.callGetOfficeGroup(req.ID, params.Arguments)
	case "get_booking_stops":
		return s.callGetBookingStops(req.ID, params.Arguments)
	case "get_booking_seats":
		return s.callGetBookingSeats(req.ID, params.Arguments)
	default:
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32601, Message: "Tool not found"},
		}
	}
}

func (s *MCPServer) callGetOriginCodes(id interface{}) MCPResponse {
	codes, err := s.client.getAllOriginCodes()
	if err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32000, Message: err.Error()},
		}
	}

	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("%v", codes),
				},
			},
		},
	}
}

func (s *MCPServer) callGetRoutes(id interface{}, args json.RawMessage) MCPResponse {
	var params struct {
		OriginCode string `json:"originCode"`
		DestCode   string `json:"destCode"`
		FromId     int    `json:"fromId"`
		ToId       int    `json:"toId"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32602, Message: "Invalid arguments"},
		}
	}

	result, err := s.client.getRoutes(getRouteRequest{
		OriginCode: params.OriginCode,
		DestCode:   params.DestCode,
		FromId:     params.FromId,
		ToId:       params.ToId,
	})

	if err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32000, Message: err.Error()},
		}
	}

	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": string(result),
				},
			},
		},
	}
}

func parseDateTime(dateStr string) (int64, error) {
	layout := "2006-01-02 15:04"
	t, err := time.Parse(layout, dateStr)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}

func (s *MCPServer) callSearchTrips(id interface{}, args json.RawMessage) MCPResponse {
	var params struct {
		RouteIds    []int  `json:"routeIds"`
		FromDate    string `json:"fromDate"`
		ToDate      string `json:"toDate"`
		TicketCount int    `json:"ticketCount"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32602, Message: "Invalid arguments"},
		}
	}

	// Parse date strings to timestamps
	fromTime, err := parseDateTime(params.FromDate)
	if err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32602, Message: fmt.Sprintf("Invalid fromDate format: %v", err)},
		}
	}

	toTime, err := parseDateTime(params.ToDate)
	if err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32602, Message: fmt.Sprintf("Invalid toDate format: %v", err)},
		}
	}

	result, err := s.client.searchTrips(params.RouteIds, fromTime, toTime, params.TicketCount)

	if err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32000, Message: err.Error()},
		}
	}

	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": string(result),
				},
			},
		},
	}
}

func (s *MCPServer) callGetPriceList(id interface{}, args json.RawMessage) MCPResponse {
	var params struct {
		RouteIds []int  `json:"routeIds"`
		FromDate string `json:"fromDate"`
		ToDate   string `json:"toDate"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32602, Message: "Invalid arguments"},
		}
	}

	result, err := s.client.getPriceListByRoutes(params.RouteIds, params.FromDate, params.ToDate)

	if err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32000, Message: err.Error()},
		}
	}

	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": string(result),
				},
			},
		},
	}
}

func (s *MCPServer) callGetOfficeGroup(id interface{}, args json.RawMessage) MCPResponse {
	var params struct {
		RouteIds []int `json:"routeIds"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32602, Message: "Invalid arguments"},
		}
	}

	result, err := s.client.getOfficeGroupByRoutes(params.RouteIds)

	if err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32000, Message: err.Error()},
		}
	}

	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": string(result),
				},
			},
		},
	}
}

func (s *MCPServer) callGetBookingStops(id interface{}, args json.RawMessage) MCPResponse {
	var params struct {
		RouteId int `json:"routeId"`
		WayId   int `json:"wayId"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32602, Message: "Invalid arguments"},
		}
	}

	result, err := s.client.getBookingStops(params.RouteId, params.WayId)

	if err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32000, Message: err.Error()},
		}
	}

	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": string(result),
				},
			},
		},
	}
}

func (s *MCPServer) callGetBookingSeats(id interface{}, args json.RawMessage) MCPResponse {
	var params struct {
		RouteId       int    `json:"routeId"`
		TripId        int    `json:"tripId"`
		DepartureDate string `json:"departureDate"`
		DepartureTime string `json:"departureTime"`
		Kind          string `json:"kind"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32602, Message: "Invalid arguments"},
		}
	}

	result, err := s.client.getBookingSeats(params.RouteId, params.TripId, params.DepartureDate, params.DepartureTime, params.Kind)

	if err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      id,
			Error:   &MCPError{Code: -32000, Message: err.Error()},
		}
	}

	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": string(result),
				},
			},
		},
	}
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
