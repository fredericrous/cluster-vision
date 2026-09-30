#!/usr/bin/env node

/**
 * cluster-vision MCP Server
 *
 * Thin HTTP client that proxies to the cluster-vision REST API.
 * Launched as a stdio MCP server by the client (e.g., Claude Desktop).
 *
 * Tools:
 * - cv_list_applications
 * - cv_get_application
 * - cv_dependency_graph
 * - cv_find_dependents
 * - cv_landscape
 * - cv_get_diagram
 * - cv_list_snapshots
 * - cv_diff
 *
 * Protocol logic lives in server.ts; this file is only the newline-delimited
 * JSON-RPC stdio transport.
 */

import { handleLine } from "./server.ts"

async function processLine(line: string) {
  // handleLine never throws for a request with an id — it turns failures
  // into JSON-RPC errors — so this guard only catches a failed stdout write.
  try {
    const response = await handleLine(line)
    if (response) process.stdout.write(JSON.stringify(response) + "\n")
  } catch (err) {
    process.stderr.write(`MCP error: ${err instanceof Error ? err.message : String(err)}\n`)
  }
}

let buffer = ""
process.stdin.setEncoding("utf-8")
process.stdin.on("data", (chunk: string) => {
  buffer += chunk
  const lines = buffer.split("\n")
  buffer = lines.pop() ?? ""
  for (const line of lines) {
    if (line.trim()) void processLine(line)
  }
})
process.stdin.on("end", () => {
  if (buffer.trim()) void processLine(buffer)
})
