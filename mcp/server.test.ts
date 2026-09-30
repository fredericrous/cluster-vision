import assert from "node:assert/strict"
import { readdirSync, readFileSync } from "node:fs"
import { join } from "node:path"
import { afterEach, describe, it } from "node:test"

import {
  DIAGRAM_IDS,
  INTERNAL_ERROR,
  INVALID_PARAMS,
  METHOD_NOT_FOUND,
  PARSE_ERROR,
  handleLine,
  tools,
} from "./server.ts"

const realFetch = globalThis.fetch
afterEach(() => {
  globalThis.fetch = realFetch
})

function stubFetch(status: number, body: string, contentType = "application/json") {
  globalThis.fetch = (async () => new Response(body, { status, headers: { "content-type": contentType } })) as typeof fetch
}

function call(id: number, name: string, args: Record<string, unknown> = {}) {
  return JSON.stringify({ jsonrpc: "2.0", id, method: "tools/call", params: { name, arguments: args } })
}

describe("DIAGRAM_IDS", () => {
  it("matches every fixed diagram ID the Go server emits", () => {
    const dir = join(import.meta.dirname, "..", "internal", "diagram")
    const goIds = new Set<string>()
    for (const file of readdirSync(dir)) {
      if (!file.endsWith(".go") || file.endsWith("_test.go")) continue
      for (const m of readFileSync(join(dir, file), "utf-8").matchAll(/\bID:\s+"([a-z][a-z0-9-]*)"/g)) {
        goIds.add(m[1])
      }
    }
    assert.deepEqual([...DIAGRAM_IDS].sort(), [...goIds].sort())
  })
})

describe("handleLine", () => {
  it("answers a parse failure with -32700 and a null id", async () => {
    const res = await handleLine("{not json")
    assert.ok(res && "error" in res)
    assert.equal(res.id, null)
    assert.equal(res.error.code, PARSE_ERROR)
  })

  it("answers an unknown method with a JSON-RPC error", async () => {
    const res = await handleLine(JSON.stringify({ jsonrpc: "2.0", id: 1, method: "nope" }))
    assert.ok(res && "error" in res)
    assert.equal(res.error.code, METHOD_NOT_FOUND)
  })

  it("answers an unknown tool with a JSON-RPC error", async () => {
    const res = await handleLine(call(2, "cv_nope"))
    assert.ok(res && "error" in res)
    assert.equal(res.id, 2)
    assert.equal(res.error.code, INVALID_PARAMS)
  })

  it("never answers a notification", async () => {
    assert.equal(await handleLine(JSON.stringify({ jsonrpc: "2.0", method: "notifications/initialized" })), null)
  })

  it("reports a plain-text 404 as a tool error naming the missing route", async () => {
    stubFetch(404, "404 page not found\n", "text/plain")
    const res = await handleLine(call(3, "cv_landscape"))
    assert.ok(res && "result" in res)
    const result = res.result as { isError?: boolean; content: Array<{ text: string }> }
    assert.equal(result.isError, true)
    assert.match(result.content[0].text, /HTTP 404/)
    assert.match(result.content[0].text, /DATABASE_URL/)
  })

  it("reports a JSON error body as a tool error, not a success", async () => {
    stubFetch(503, JSON.stringify({ error: "no cluster data available yet" }))
    const res = await handleLine(call(4, "cv_get_diagram", { type: "charts" }))
    assert.ok(res && "result" in res)
    const result = res.result as { isError?: boolean; content: Array<{ text: string }> }
    assert.equal(result.isError, true)
    assert.match(result.content[0].text, /HTTP 503: no cluster data available yet/)
  })

  it("returns a matching diagram", async () => {
    stubFetch(200, JSON.stringify({ diagrams: [{ id: "charts", title: "Charts" }] }))
    const res = await handleLine(call(5, "cv_get_diagram", { type: "charts" }))
    assert.ok(res && "result" in res)
    const result = res.result as { isError?: boolean; content: Array<{ text: string }> }
    assert.equal(result.isError, undefined)
    assert.deepEqual(JSON.parse(result.content[0].text), { id: "charts", title: "Charts" })
  })

  it("answers an internal failure with -32603 when the id is known", async () => {
    tools.__broken = {
      get description(): string {
        throw new Error("boom")
      },
      inputSchema: {},
      handler: async () => null,
    }
    try {
      const res = await handleLine(JSON.stringify({ jsonrpc: "2.0", id: 6, method: "tools/list" }))
      assert.ok(res && "error" in res)
      assert.equal(res.id, 6)
      assert.equal(res.error.code, INTERNAL_ERROR)
      assert.match(res.error.message, /boom/)
    } finally {
      delete tools.__broken
    }
  })
})
