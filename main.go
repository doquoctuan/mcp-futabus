package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
)

type futabusClient struct {
}

// NewFutabusClient creates a new instance of futabusClient
func NewFutabusClient() *futabusClient {
	return &futabusClient{}
}

func (c *futabusClient) getToken() string {
	resp, err := http.Get("https://futabus.vn")
	if err != nil {
		fmt.Printf("Error fetching page: %v\n", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return ""
	}

	html := string(body)

	// Extract JSON from __NEXT_DATA__ script tag
	re := regexp.MustCompile(`<script id="__NEXT_DATA__" type="application/json">([^<]*)</script>`)
	matches := re.FindStringSubmatch(html)

	if len(matches) < 2 {
		fmt.Println("Could not find __NEXT_DATA__ script tag")
		return ""
	}

	// Parse the JSON data
	var data struct {
		Props struct {
			PageProps struct {
				Token string `json:"token"`
			} `json:"pageProps"`
		} `json:"props"`
	}

	err = json.Unmarshal([]byte(matches[1]), &data)
	if err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		return ""
	}

	return data.Props.PageProps.Token
}

func (c *futabusClient) searchTrips(routeIDs []int, fromTime, toTime int64, ticketCount int) ([]byte, error) {
	endpoint := "https://api.futabus.vn/search/trips"

	requestBody := map[string]interface{}{
		"channel":          "web_client",
		"size":             300,
		"only_online_trip": true,
		"ticket_count":     ticketCount,
		"seat_type_id":     []int{},
		"from_time":        fromTime,
		"to_time":          toTime,
		"route_ids":        routeIDs,
		"origin_office_id": []int{},
		"dest_office_id":   []int{},
		"postion":          []int{},
		"floor":            []int{},
		"sort_by":          []string{"departure_time"},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling JSON: %v", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.getToken()))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return body, nil
}

type getRouteRequest struct {
	ToId           int
	FromId         int
	OriginCode     string
	DestCode       string
	SkipCount      int
	MaxResultCount int
}

func (c *futabusClient) getRoutes(routeRequest getRouteRequest) ([]byte, error) {
	endpoint := "https://api.futabus.vn/metadata/office/routes/v2"

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set default values if not provided
	if routeRequest.SkipCount == 0 {
		routeRequest.SkipCount = 0
	}
	if routeRequest.MaxResultCount == 0 {
		routeRequest.MaxResultCount = 100
	}

	// Add query parameters
	q := req.URL.Query()
	q.Add("SkipCount", fmt.Sprintf("%d", routeRequest.SkipCount))
	q.Add("MaxResultCount", fmt.Sprintf("%d", routeRequest.MaxResultCount))
	q.Add("DestCode", routeRequest.DestCode)
	q.Add("FromId", fmt.Sprintf("%d", routeRequest.FromId))
	q.Add("OriginCode", routeRequest.OriginCode)
	q.Add("ToId", fmt.Sprintf("%d", routeRequest.ToId))
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return body, nil
}

func (c *futabusClient) getPriceListByRoutes(routeIDs []int, fromDate, toDate string) ([]byte, error) {
	endpoint := "https://api.futabus.vn/ticket-base/api/price/minimum-by-list-routes"

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Add query parameters
	q := req.URL.Query()
	for _, routeID := range routeIDs {
		q.Add("route_ids", fmt.Sprintf("%d", routeID))
	}
	q.Add("from_date", fromDate)
	q.Add("to_date", toDate)
	req.URL.RawQuery = q.Encode()

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.getToken()))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return body, nil
}

type responseOriginCode struct {
	Id    int    `json:"id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	Tags  string `json:"tags"`
	Level int    `json:"level"`
	Code  string `json:"code"`
}

func (c *futabusClient) getAllOriginCodes() ([]responseOriginCode, error) {
	endpoint := "https://api.futabus.vn/metadata/search/origin-codes"

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.getToken()))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	var codes []responseOriginCode
	if err := json.Unmarshal(body, &codes); err != nil {
		return nil, fmt.Errorf("error unmarshaling origin codes: %v", err)
	}

	return codes, nil
}

func (c *futabusClient) getOfficeGroupByRoutes(routeIDs []int) ([]byte, error) {
	endpoint := "https://api.futabus.vn/metadata/office/office-group"

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Add query parameters
	q := req.URL.Query()
	for _, routeID := range routeIDs {
		q.Add("RouteIds", fmt.Sprintf("%d", routeID))
	}
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return body, nil
}

func (c *futabusClient) getBookingStops(routeID int, wayID int) ([]byte, error) {
	endpoint := fmt.Sprintf("https://api-busline.vato.vn/api/buslines/futa/booking/stops/%d", routeID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Add query parameters
	q := req.URL.Query()
	q.Add("wayId", fmt.Sprintf("%d", wayID))
	req.URL.RawQuery = q.Encode()

	// Add headers
	req.Header.Set("token_type", "anonymous")
	req.Header.Set("x-access-token", c.getToken())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return body, nil
}

func (c *futabusClient) getBookingSeats(routeID int, tripID int, departureDate, departureTime, kind string) ([]byte, error) {
	endpoint := fmt.Sprintf("https://api-busline.vato.vn/api/buslines/futa/booking/seats/%d/%d", routeID, tripID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Add query parameters
	q := req.URL.Query()
	q.Add("departureDate", departureDate)
	q.Add("departureTime", departureTime)
	q.Add("kind", kind)
	req.URL.RawQuery = q.Encode()

	// Add headers
	req.Header.Set("token_type", "anonymous")
	req.Header.Set("x-access-token", c.getToken())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return body, nil
}

func main() {
	server := NewMCPServer()
	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
