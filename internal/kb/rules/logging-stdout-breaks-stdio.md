---
title: Console logger writes to STDOUT and corrupts the stdio MCP stream
package: log
tags: [logging, stdio, mcp, transport]
use_instead: a file or STDERR log driver (never the console/stdout driver in a stdio server)
deprecated: false
---
A stdio MCP server speaks JSON-RPC over STDOUT. velocity's console log driver
also writes to STDOUT, so any log line interleaves with protocol frames and
corrupts the stream, which the client rejects as malformed JSON-RPC.

In a stdio server, configure logging to a file driver (or STDERR) before serving.
Diagnostics belong on STDERR; STDOUT carries protocol bytes only.
