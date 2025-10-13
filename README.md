# MCP Futabus Server

A Model Context Protocol (MCP) server that provides access to Futabus bus booking services. This server allows you to search for bus routes, trips, pricing, and booking information through a standardized interface.

## Features

The MCP server provides the following tools:

- **get_origin_codes**: Get all available origin codes for bus routes
- **get_routes**: Get routes between origin and destination points
- **search_trips**: Search for available trips within a date range
- **get_price_list**: Get minimum prices for routes within a date range
- **get_office_group**: Get office groups for specific routes
- **get_booking_stops**: Get booking stops for a specific route
- **get_booking_seats**: Get available seats for a specific trip

## Usage

The server communicates via JSON-RPC 2.0 over stdin/stdout and implements the MCP protocol version 2024-11-05.

### Example Tools

1. **Search Routes**: Find available routes between two locations
   ```json
   {
     "name": "get_routes",
     "arguments": {
       "originCode": "HCM",
       "destCode": "HN",
       "fromId": 1,
       "toId": 2
     }
   }
   ```

2. **Search Trips**: Find available trips with date/time constraints
   ```json
   {
     "name": "search_trips",
     "arguments": {
       "routeIds": [1, 2, 3],
       "fromDate": "2024-01-01 08:00",
       "toDate": "2024-01-02 20:00",
       "ticketCount": 2
     }
   }
   ```

3. **Get Price List**: Get pricing information for routes
   ```json
   {
     "name": "get_price_list",
     "arguments": {
       "routeIds": [1, 2],
       "fromDate": "01-01-2024",
       "toDate": "02-01-2024"
     }
   }
   ```

## How to expose MCP server for n8n Cloud

This guide shows how to run your local MCP server, proxy it, and expose it securely for use inside n8n Cloud.

### Prerequisites
- Go installed (go version to verify)
- mcp-proxy installed (brew install mcp-proxy)
- ngrok account and auth token (ngrok config add-authtoken <TOKEN>)

### 1. Start (or prepare) your local MCP server
If your MCP server is implemented in Go and starts with `go run .`, you do not run it directly; mcp-proxy will start it for you.

### 2. Run mcp-proxy locally
The proxy will launch your MCP server and listen on port 8080:

```
mcp-proxy --port=8080 -- go run .
```

```
ngrok http 8080
```