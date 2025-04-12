# d2-mcp

## Running

Compile

```bash
go build .
```

## Configuring clients

MacOS:

```bash
# Claude Desktop
$EDITOR ~/Library/Application\ Support/Claude/claude_desktop_config.json
# OTerm:
$EDITOR ~/Library/Application\ Support/oterm/config.json
```

Compile the server and add the following:

```json
{
    "mcpServers": {
        "d2": {
            "command": "/YOUR/ABSOLUTE/PATH/d2-mcp/d2-mcp",
            "args": ["--image-type", "png"]
        }
    }
}
```

## Debugging

```bash
npx @modelcontextprotocol/inspector /YOUR/ABSOLUTE/PATH/d2-mcp/d2-mcp
```