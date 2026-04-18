package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"
)

const (
	baseURL    = "https://api-online.futabus.vn"
	webURL     = "https://futabus.vn"
	userAgent  = "FUTA/7.36.4 (com.client.facecar; build:1; iOS 26.2.0) Alamofire/5.9.1"
	appVersion = "7.36.4"
	channel    = "mobile_app"
)

type futabusClient struct {
	httpClient *http.Client
	token      string
	tokenTime  time.Time
}

func NewFutabusClient() *futabusClient {
	return &futabusClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *futabusClient) getToken(ctx context.Context) (string, error) {
	if c.token != "" && time.Since(c.tokenTime) < 30*time.Minute {
		return c.token, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", webURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch futabus.vn: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`"token"\s*:\s*"([^"]+)"`)
	matches := re.FindSubmatch(body)
	if len(matches) < 2 {
		return "", fmt.Errorf("token not found in response")
	}

	c.token = string(matches[1])
	c.tokenTime = time.Now()
	return c.token, nil
}

func (c *futabusClient) doRequest(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	fullURL := baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-App-Version", appVersion)
	req.Header.Set("X-Access-Token", token)
	req.Header.Set("X-Channel", channel)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// API Response types

type APIResponse struct {
	RequestID string          `json:"requestId"`
	Status    int             `json:"status"`
	Error     json.RawMessage `json:"error"`
	Data      json.RawMessage `json:"data"`
}

type PaginationData struct {
	Page   int               `json:"page"`
	Size   int               `json:"size"`
	Total  int               `json:"total"`
	Items  []json.RawMessage `json:"items"`
	Others []json.RawMessage `json:"others"`
}

type ListData struct {
	Items  []json.RawMessage `json:"items"`
	Others []json.RawMessage `json:"others"`
}

type PickupPointGroup struct {
	DistrictID   string        `json:"districtId"`
	DistrictName string        `json:"districtName"`
	ProvinceName string        `json:"provinceName"`
	Group        []PickupPoint `json:"group"`
}

type PickupPoint struct {
	DepartmentID      string  `json:"departmentId"`
	DepartmentName    string  `json:"departmentName"`
	DepartmentAddress string  `json:"departmentAddress"`
	DepartmentTime    int     `json:"departmentTime"`
	AreaID            string  `json:"areaId"`
	ProvinceID        string  `json:"provinceId"`
	ProvinceName      string  `json:"provinceName"`
	DistrictID        string  `json:"districtId"`
	DistrictName      string  `json:"districtName"`
	Type              int     `json:"type"`
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
}

type Area struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

type RouteSearchData struct {
	RouteID string `json:"routeId"`
	From    string `json:"from"`
	To      string `json:"to"`
}

type TripSearchData struct {
	TripID             string `json:"tripId"`
	DepartureTime      string `json:"departureTime"`
	RawDepartureTime   string `json:"rawDepartureTime"`
	RawDepartureDate   string `json:"rawDepartureDate"`
	ArrivalTime        string `json:"arrivalTime"`
	Duration           int    `json:"duration"`
	SeatTypeName       string `json:"seatTypeName"`
	Price              int    `json:"price"`
	EmptySeatQuantity  int    `json:"emptySeatQuantity"`
	RouteID            string `json:"routeId"`
	Distance           int    `json:"distance"`
	WayID              string `json:"wayId"`
	MaxSeatsPerBooking int    `json:"maxSeatsPerBooking"`
	WayName            string `json:"wayName"`
	Route              Route  `json:"route"`
	SeatTypeCode       string `json:"seatTypeCode"`
}

type Route struct {
	OriginCode string `json:"originCode"`
	DestCode   string `json:"destCode"`
	OriginName string `json:"originName"`
	DestName   string `json:"destName"`
	Name       string `json:"name"`
	OriginHub  string `json:"originHubName"`
	DestHub    string `json:"destHubName"`
}

type SeatDiagramData struct {
	SeatID   string `json:"seatId"`
	Name     string `json:"name"`
	Status   []int  `json:"status"`
	ColumnNo int    `json:"columnNo"`
	RowNo    int    `json:"rowNo"`
	Floor    string `json:"floor"`
	Price    int    `json:"price"`
}

type DepartmentInWay struct {
	DepartmentID      string  `json:"departmentId"`
	DepartmentName    string  `json:"departmentName"`
	DepartmentAddress string  `json:"departmentAddress"`
	TimeAtDepartment  int     `json:"timeAtDepartment"`
	Passing           bool    `json:"passing"`
	IsShuttleService  bool    `json:"isShuttleService"`
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
	PointKind         int     `json:"pointKind"`
	PresentBeforeMins int     `json:"presentBeforeMinutes"`
}

func (c *futabusClient) SearchPickupPoints(ctx context.Context, keyword string) ([]PickupPointGroup, []Area, error) {
	path := fmt.Sprintf("/vato/v1/search/pickup-point?keyword=%s&page=0&size=50",
		url.QueryEscape(keyword))

	data, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, nil, err
	}
	if resp.Status != 200 {
		return nil, nil, fmt.Errorf("API error status %d: %s", resp.Status, string(resp.Error))
	}

	var pg PaginationData
	if err := json.Unmarshal(resp.Data, &pg); err != nil {
		return nil, nil, err
	}

	var groups []PickupPointGroup
	for _, item := range pg.Items {
		var g PickupPointGroup
		if err := json.Unmarshal(item, &g); err != nil {
			continue
		}
		groups = append(groups, g)
	}

	var areas []Area
	for _, other := range pg.Others {
		var a Area
		if err := json.Unmarshal(other, &a); err != nil {
			continue
		}
		areas = append(areas, a)
	}

	return groups, areas, nil
}

func (c *futabusClient) SearchRoutes(ctx context.Context, originAreaID, destAreaID, fromDate string) ([]RouteSearchData, error) {
	path := fmt.Sprintf("/vato/v1/search/routes?destAreaId=%s&destOfficeId=&originAreaId=%s&originOfficeId=&isReturn=false&isReturnTripLoad=false&fromDate=%s",
		url.QueryEscape(destAreaID),
		url.QueryEscape(originAreaID),
		url.QueryEscape(fromDate))

	data, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("routes API error: %s", string(resp.Error))
	}

	var ld ListData
	if err := json.Unmarshal(resp.Data, &ld); err != nil {
		return nil, err
	}

	var routes []RouteSearchData
	for _, item := range ld.Items {
		var r RouteSearchData
		if err := json.Unmarshal(item, &r); err != nil {
			continue
		}
		routes = append(routes, r)
	}
	return routes, nil
}

func (c *futabusClient) SearchTripsByRoute(ctx context.Context, routeIDs []string, fromDate, toDate string) ([]TripSearchData, error) {
	type reqBody struct {
		MinNumSeat int      `json:"minNumSeat"`
		Channel    string   `json:"channel"`
		FromDate   string   `json:"fromDate"`
		ToDate     string   `json:"toDate"`
		RouteIDs   []string `json:"routeIds"`
		Sort       struct {
			ByPrice         string `json:"byPrice"`
			ByDepartureTime string `json:"byDepartureTime"`
		} `json:"sort"`
		Page int `json:"page"`
		Size int `json:"size"`
	}

	rb := reqBody{
		MinNumSeat: 1,
		Channel:    channel,
		FromDate:   fromDate,
		ToDate:     toDate,
		RouteIDs:   routeIDs,
		Page:       0,
		Size:       200,
	}
	rb.Sort.ByPrice = "asc"
	rb.Sort.ByDepartureTime = "asc"

	bodyBytes, err := json.Marshal(rb)
	if err != nil {
		return nil, err
	}

	data, err := c.doRequest(ctx, "POST", "/vato/v1/search/trip-by-route", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("trips API error: %s", string(resp.Error))
	}

	var pg PaginationData
	if err := json.Unmarshal(resp.Data, &pg); err != nil {
		return nil, err
	}

	var trips []TripSearchData
	for _, item := range pg.Items {
		var t TripSearchData
		if err := json.Unmarshal(item, &t); err != nil {
			continue
		}
		trips = append(trips, t)
	}
	return trips, nil
}

func (c *futabusClient) GetSeatDiagram(ctx context.Context, tripID string) ([]SeatDiagramData, error) {
	path := fmt.Sprintf("/vato/v1/search/seat-diagram/%s", tripID)

	data, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("seat diagram API error: %s", string(resp.Error))
	}

	var pg PaginationData
	if err := json.Unmarshal(resp.Data, &pg); err != nil {
		return nil, err
	}

	var seats []SeatDiagramData
	for _, item := range pg.Items {
		var s SeatDiagramData
		if err := json.Unmarshal(item, &s); err != nil {
			continue
		}
		seats = append(seats, s)
	}
	return seats, nil
}

func (c *futabusClient) GetDepartmentsInWay(ctx context.Context, wayID, routeID string) ([]DepartmentInWay, error) {
	path := fmt.Sprintf("/vato/v1/search/department-in-way/%s?routeId=%s", wayID, routeID)

	data, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("dept-in-way API error: %s", string(resp.Error))
	}

	var pg PaginationData
	if err := json.Unmarshal(resp.Data, &pg); err != nil {
		return nil, err
	}

	var depts []DepartmentInWay
	for _, item := range pg.Items {
		var d DepartmentInWay
		if err := json.Unmarshal(item, &d); err != nil {
			continue
		}
		depts = append(depts, d)
	}
	return depts, nil
}

func main() {
	httpAddr := flag.String("http", "", "HTTP address to listen on for Streamable HTTP transport (e.g. :8080). STDIO is always enabled.")
	flag.Parse()

	server := NewMCPServer()

	if *httpAddr != "" {
		if err := server.RunHTTP(*httpAddr); err != nil {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
