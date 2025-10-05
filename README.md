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