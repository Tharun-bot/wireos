import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  Tool,
  TextContent,
} from "@modelcontextprotocol/sdk/types.js";

// ── config ────────────────────────────────────────────────────
const WIREOS_API = process.env.WIREOS_API_URL ?? "http://localhost:8081";
const SERVER_NAME = "wireos";
const SERVER_VERSION = "0.1.0";

// ── intent metadata (mirrors intents.yaml) ────────────────────
const INTENT_IDS = [
  "recent_purchases",
  "professional_activity",
  "github_activity",
  "portfolio_snapshot",
  "job_applications",
  "web_research",
] as const;

type IntentID = (typeof INTENT_IDS)[number];

// ── tool definition ───────────────────────────────────────────
const WIREOS_INTENT_TOOL: Tool = {
  name: "wireOS_intent",
  description: [
    "Query across all your authenticated accounts via a single intent.",
    "WireOS fans out to multiple data sources in parallel and returns a unified, normalized result.",
    "",
    "Available intents:",
    "  • recent_purchases      — What did I buy recently? (sources: Amazon)",
    "  • professional_activity — What have I been doing professionally? (sources: LinkedIn)",
    "  • github_activity       — Recent commits, PRs, and code reviews (sources: GitHub)",
    "  • portfolio_snapshot    — Current investment portfolio and net worth (sources: Robinhood)",
    "  • job_applications      — Jobs I've applied to and their status (sources: LinkedIn)",
    "  • web_research          — Search the web for current information (sources: Anakin search)",
    "",
    "For web_research, pass params: { query: 'your search query' }",
    "For github_activity, pass params: { username: 'your-github-username' }",
  ].join("\n"),
  inputSchema: {
    type: "object",
    properties: {
      intent_id: {
        type: "string",
        enum: [...INTENT_IDS],
        description: "The intent to execute. Must be one of the available intent IDs listed above.",
      },
      params: {
        type: "object",
        description:
          "Optional parameters to pass to the intent. " +
          "For web_research: { query: string }. " +
          "For github_activity: { username: string }. " +
          "Other intents work with no params.",
        additionalProperties: true,
      },
    },
    required: ["intent_id"],
  },
};

// ── WireOS API client ─────────────────────────────────────────

interface IntentRequest {
  intent_id: string;
  params?: Record<string, string>;
}

interface NormalizedResult {
  output_type: string;
  source: string;
  error?: string;
  latency_ms: number;
  transactions?: unknown[];
  activities?: unknown[];
  contacts?: unknown[];
  generic?: Record<string, unknown>;
}

interface IntentResponse {
  intent_id: string;
  label: string;
  results: NormalizedResult[];
  partial_failure: boolean;
  total_latency_ms: number;
}

interface ErrorResponse {
  error: string;
}

async function callWireOS(req: IntentRequest): Promise<IntentResponse> {
  const url = `${WIREOS_API}/intent`;

  let res: Response;
  try {
    res = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(req),
      signal: AbortSignal.timeout(30_000), // 30s — matches backend ceiling
    });
  } catch (err) {
    if (err instanceof Error && err.name === "TimeoutError") {
      throw new Error(`WireOS API timed out after 30s (${url})`);
    }
    throw new Error(
      `WireOS API unreachable at ${url}: ${err instanceof Error ? err.message : String(err)}`
    );
  }

  const body = await res.json() as IntentResponse | ErrorResponse;

  if (!res.ok) {
    const msg = "error" in body ? body.error : `HTTP ${res.status}`;
    throw new Error(`WireOS API error: ${msg}`);
  }

  return body as IntentResponse;
}

// ── result formatter ──────────────────────────────────────────
// Converts the structured IntentResponse into readable prose for Claude.

function formatResponse(data: IntentResponse): string {
  const lines: string[] = [];

  lines.push(`# WireOS — ${data.label}`);
  lines.push(`Intent: ${data.intent_id} | Latency: ${data.total_latency_ms}ms | Sources: ${data.results.length}`);
  if (data.partial_failure) {
    lines.push("⚠️  Partial failure: one or more sources failed. Results below are from sources that succeeded.");
  }
  lines.push("");

  if (data.results.length === 0) {
    lines.push("No results returned.");
    return lines.join("\n");
  }

  for (const result of data.results) {
    lines.push(`## Source: ${result.source} (${result.output_type}) — ${result.latency_ms}ms`);

    if (result.error) {
      lines.push(`❌ Error: ${result.error}`);
      lines.push("");
      continue;
    }

    switch (result.output_type) {
      case "transaction":
        if (result.transactions && result.transactions.length > 0) {
          lines.push(`${result.transactions.length} transaction(s):`);
          lines.push("```json");
          lines.push(JSON.stringify(result.transactions, null, 2));
          lines.push("```");
        } else {
          lines.push("No transactions found.");
        }
        break;

      case "activity":
        if (result.activities && result.activities.length > 0) {
          lines.push(`${result.activities.length} activity item(s):`);
          lines.push("```json");
          lines.push(JSON.stringify(result.activities, null, 2));
          lines.push("```");
        } else {
          lines.push("No activity found.");
        }
        break;

      case "contact":
        if (result.contacts && result.contacts.length > 0) {
          lines.push(`${result.contacts.length} contact(s):`);
          lines.push("```json");
          lines.push(JSON.stringify(result.contacts, null, 2));
          lines.push("```");
        } else {
          lines.push("No contacts found.");
        }
        break;

      default:
        // "generic" or unknown — dump as JSON
        if (result.generic && Object.keys(result.generic).length > 0) {
          lines.push("```json");
          lines.push(JSON.stringify(result.generic, null, 2));
          lines.push("```");
        } else {
          lines.push("No data returned.");
        }
        break;
    }

    lines.push("");
  }

  // Raw JSON at the end for Claude to reference programmatically if needed
  lines.push("---");
  lines.push("<raw_json>");
  lines.push(JSON.stringify(data, null, 2));
  lines.push("</raw_json>");

  return lines.join("\n");
}

// ── MCP server setup ──────────────────────────────────────────

function createServer(): Server {
  const server = new Server(
    { name: SERVER_NAME, version: SERVER_VERSION },
    { capabilities: { tools: {} } }
  );

  // List tools handler
  server.setRequestHandler(ListToolsRequestSchema, async () => {
    return { tools: [WIREOS_INTENT_TOOL] };
  });

  // Call tool handler
  server.setRequestHandler(CallToolRequestSchema, async (request) => {
    const { name, arguments: args } = request.params;

    if (name !== "wireOS_intent") {
      return {
        content: [
          {
            type: "text",
            text: `Unknown tool: ${name}`,
          } satisfies TextContent,
        ],
        isError: true,
      };
    }

    // Validate intent_id
    const intentId = args?.intent_id as string | undefined;
    if (!intentId) {
      return {
        content: [
          {
            type: "text",
            text: "Missing required parameter: intent_id",
          } satisfies TextContent,
        ],
        isError: true,
      };
    }

    if (!INTENT_IDS.includes(intentId as IntentID)) {
      return {
        content: [
          {
            type: "text",
            text: [
              `Unknown intent_id: "${intentId}"`,
              `Valid intents: ${INTENT_IDS.join(", ")}`,
            ].join("\n"),
          } satisfies TextContent,
        ],
        isError: true,
      };
    }

    // Coerce params — the Go backend expects map[string]string
    const rawParams = (args?.params ?? {}) as Record<string, unknown>;
    const params: Record<string, string> = {};
    for (const [k, v] of Object.entries(rawParams)) {
      params[k] = String(v);
    }

    try {
      const data = await callWireOS({ intent_id: intentId, params });
      const text = formatResponse(data);
      return {
        content: [{ type: "text", text } satisfies TextContent],
        isError: false,
      };
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      return {
        content: [
          {
            type: "text",
            text: `WireOS error: ${message}`,
          } satisfies TextContent,
        ],
        isError: true,
      };
    }
  });

  return server;
}

// ── main ──────────────────────────────────────────────────────
async function main(): Promise<void> {
  const server = createServer();
  const transport = new StdioServerTransport();

  // Graceful shutdown
  process.on("SIGINT",  () => { server.close(); process.exit(0); });
  process.on("SIGTERM", () => { server.close(); process.exit(0); });

  await server.connect(transport);

  // MCP servers communicate over stdio — log to stderr so stdout stays clean
  process.stderr.write(
    `[wireos-mcp] server started — connected to ${WIREOS_API}\n`
  );
}

main().catch((err) => {
  process.stderr.write(`[wireos-mcp] fatal: ${err}\n`);
  process.exit(1);
});