export type IntegrationCategory = "apm" | "metrics" | "logs"

export interface Integration {
  slug: string
  name: string
  description: string
  icon: string           // Lucide icon name
  category: IntegrationCategory
  dataType: "Traces" | "Metrics" | "Logs"
  setupNotes: string     // per-integration setup description for Step 2
  prerequisites?: string[] // requirements shown before setup
  verifyPath: string     // OTLP verify endpoint path, e.g. /v1/traces
  dockerSnippet: string  // otelcol docker-compose config, uses {{TOKEN}} and {{ENDPOINT}}
  binarySnippet: string  // otelcol config.yaml for binary install
  sdkSnippet?: string    // APM-only: SDK init code
  sdkLang?: string       // language for syntax highlighting
}

const OTELCOL_IMAGE = "otel/opentelemetry-collector-contrib:0.101.0"

// Shared exporter config block reused across snippets
const exporterBlock = (dataType: "traces" | "metrics" | "logs") => `
  otlphttp/${dataType}:
    endpoint: "{{ENDPOINT}}"
    headers:
      Authorization: "Bearer {{TOKEN}}"`

export const integrations: Integration[] = [
  // ── APM ──────────────────────────────────────────────────────────────────

  {
    slug: "go",
    name: "Go",
    description: "Instrument Go applications with the OpenTelemetry Go SDK",
    icon: "Cpu",
    category: "apm",
    dataType: "Traces",
    setupNotes: "Add the OpenTelemetry Go SDK to your application. Initialize the tracer provider at startup and it will automatically instrument HTTP clients, gRPC, and database calls. No separate collector is needed — traces are exported directly from your app.",
    prerequisites: ["Go 1.21+", "go.opentelemetry.io/otel v1.24+"],
    verifyPath: "/v1/traces",
    sdkLang: "go",
    sdkSnippet: `import (
  "go.opentelemetry.io/otel"
  "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
  sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func initTracer(ctx context.Context) (func(), error) {
  exp, err := otlptracehttp.New(ctx,
    otlptracehttp.WithEndpointURL("{{ENDPOINT}}/v1/traces"),
    otlptracehttp.WithHeaders(map[string]string{
      "Authorization": "Bearer {{TOKEN}}",
    }),
  )
  if err != nil {
    return nil, err
  }
  tp := sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(exp),
    sdktrace.WithResource(resource.NewWithAttributes(
      semconv.SchemaURL,
      semconv.ServiceName("my-service"),
    )),
  )
  otel.SetTracerProvider(tp)
  return func() { tp.Shutdown(ctx) }, nil
}`,
    dockerSnippet: `# No collector needed for SDK-based APM.
# Configure your app environment variables:
services:
  my-app:
    environment:
      OTEL_EXPORTER_OTLP_ENDPOINT: "{{ENDPOINT}}"
      OTEL_EXPORTER_OTLP_HEADERS: "Authorization=Bearer {{TOKEN}}"
      OTEL_SERVICE_NAME: "my-service"`,
    binarySnippet: `# Set environment variables before running your Go app:
export OTEL_EXPORTER_OTLP_ENDPOINT="{{ENDPOINT}}"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer {{TOKEN}}"
export OTEL_SERVICE_NAME="my-service"`,
  },

  {
    slug: "nodejs",
    name: "Node.js",
    description: "Instrument Node.js applications with the OpenTelemetry JS SDK",
    icon: "Hexagon",
    category: "apm",
    dataType: "Traces",
    setupNotes: "Create a tracing.js file and require it before your application entry point. The SDK auto-instruments Express, Fastify, HTTP, and most popular frameworks. Use --require or NODE_OPTIONS to load it.",
    prerequisites: ["Node.js 18+", "@opentelemetry/sdk-node", "@opentelemetry/exporter-trace-otlp-http"],
    verifyPath: "/v1/traces",
    sdkLang: "javascript",
    sdkSnippet: `// tracing.js — require this before your app
const { NodeSDK } = require('@opentelemetry/sdk-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-http');
const { Resource } = require('@opentelemetry/resources');
const { SemanticResourceAttributes } = require('@opentelemetry/semantic-conventions');

const sdk = new NodeSDK({
  resource: new Resource({
    [SemanticResourceAttributes.SERVICE_NAME]: 'my-service',
  }),
  traceExporter: new OTLPTraceExporter({
    url: '{{ENDPOINT}}/v1/traces',
    headers: { Authorization: 'Bearer {{TOKEN}}' },
  }),
});

sdk.start();`,
    dockerSnippet: `services:
  my-app:
    environment:
      OTEL_EXPORTER_OTLP_ENDPOINT: "{{ENDPOINT}}"
      OTEL_EXPORTER_OTLP_HEADERS: "Authorization=Bearer {{TOKEN}}"
      OTEL_SERVICE_NAME: "my-service"
      NODE_OPTIONS: "--require ./tracing.js"`,
    binarySnippet: `export OTEL_EXPORTER_OTLP_ENDPOINT="{{ENDPOINT}}"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer {{TOKEN}}"
export OTEL_SERVICE_NAME="my-service"
node --require ./tracing.js app.js`,
  },

  {
    slug: "python",
    name: "Python",
    description: "Instrument Python applications with the OpenTelemetry Python SDK",
    icon: "Terminal",
    category: "apm",
    dataType: "Traces",
    setupNotes: "Install the OpenTelemetry distro and OTLP exporter packages. You can either initialize the tracer in code or use the opentelemetry-instrument CLI wrapper for zero-code instrumentation of Django, Flask, and FastAPI.",
    prerequisites: ["Python 3.8+", "opentelemetry-distro", "opentelemetry-exporter-otlp"],
    verifyPath: "/v1/traces",
    sdkLang: "python",
    sdkSnippet: `# pip install opentelemetry-distro opentelemetry-exporter-otlp
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter

exporter = OTLPSpanExporter(
    endpoint="{{ENDPOINT}}/v1/traces",
    headers={"Authorization": "Bearer {{TOKEN}}"},
)
provider = TracerProvider()
provider.add_span_processor(BatchSpanProcessor(exporter))
trace.set_tracer_provider(provider)`,
    dockerSnippet: `services:
  my-app:
    environment:
      OTEL_EXPORTER_OTLP_ENDPOINT: "{{ENDPOINT}}"
      OTEL_EXPORTER_OTLP_HEADERS: "Authorization=Bearer {{TOKEN}}"
      OTEL_SERVICE_NAME: "my-service"`,
    binarySnippet: `export OTEL_EXPORTER_OTLP_ENDPOINT="{{ENDPOINT}}"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer {{TOKEN}}"
export OTEL_SERVICE_NAME="my-service"
opentelemetry-instrument python app.py`,
  },

  {
    slug: "java",
    name: "Java",
    description: "Instrument Java applications with the OpenTelemetry Java Agent",
    icon: "Coffee",
    category: "apm",
    dataType: "Traces",
    setupNotes: "Download the OpenTelemetry Java Agent JAR and attach it to your JVM via -javaagent flag. The agent provides zero-code auto-instrumentation for Spring Boot, JDBC, Servlet, gRPC, and 100+ libraries.",
    prerequisites: ["Java 8+", "opentelemetry-javaagent.jar"],
    verifyPath: "/v1/traces",
    sdkLang: "bash",
    sdkSnippet: `# Download the Java agent
curl -L https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar -o opentelemetry-javaagent.jar

# Run your app with the agent
java -javaagent:opentelemetry-javaagent.jar \\
  -Dotel.exporter.otlp.endpoint="{{ENDPOINT}}" \\
  -Dotel.exporter.otlp.headers="Authorization=Bearer {{TOKEN}}" \\
  -Dotel.service.name="my-service" \\
  -jar my-app.jar`,
    dockerSnippet: `services:
  my-app:
    environment:
      OTEL_EXPORTER_OTLP_ENDPOINT: "{{ENDPOINT}}"
      OTEL_EXPORTER_OTLP_HEADERS: "Authorization=Bearer {{TOKEN}}"
      OTEL_SERVICE_NAME: "my-service"
      JAVA_TOOL_OPTIONS: "-javaagent:/app/opentelemetry-javaagent.jar"`,
    binarySnippet: `export OTEL_EXPORTER_OTLP_ENDPOINT="{{ENDPOINT}}"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer {{TOKEN}}"
export OTEL_SERVICE_NAME="my-service"
java -javaagent:opentelemetry-javaagent.jar -jar my-app.jar`,
  },

  // ── Metrics ──────────────────────────────────────────────────────────────

  {
    slug: "host",
    name: "Host",
    description: "Collect CPU, memory, disk, and network metrics from the host",
    icon: "Monitor",
    category: "metrics",
    dataType: "Metrics",
    setupNotes: "Deploy the OpenTelemetry Collector with the hostmetrics receiver. The collector reads from /proc and /sys to collect CPU usage, memory utilization, disk I/O, filesystem usage, network traffic, and system load averages.",
    prerequisites: ["Linux host (or macOS for dev)", "Access to /proc and /sys"],
    verifyPath: "/v1/metrics",
    dockerSnippet: `services:
  otelcol:
    image: ${OTELCOL_IMAGE}
    volumes:
      - ./otelcol-host.yaml:/etc/otelcol-contrib/config.yaml
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
    network_mode: host
    restart: unless-stopped`,
    binarySnippet: `# /etc/otelcol/config.yaml
receivers:
  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu: {}
      memory: {}
      disk: {}
      filesystem: {}
      network: {}
      load: {}

exporters:${exporterBlock("metrics")}

service:
  pipelines:
    metrics:
      receivers: [hostmetrics]
      exporters: [otlphttp/metrics]`,
  },

  {
    slug: "docker",
    name: "Docker",
    description: "Collect container CPU, memory, and network metrics via Docker stats",
    icon: "Box",
    category: "metrics",
    dataType: "Metrics",
    setupNotes: "Deploy the OpenTelemetry Collector with the docker_stats receiver. It connects to the Docker daemon via the Unix socket to collect per-container CPU, memory, block I/O, and network metrics.",
    prerequisites: ["Docker Engine running", "Read access to /var/run/docker.sock"],
    verifyPath: "/v1/metrics",
    dockerSnippet: `services:
  otelcol:
    image: ${OTELCOL_IMAGE}
    volumes:
      - ./otelcol-docker.yaml:/etc/otelcol-contrib/config.yaml
      - /var/run/docker.sock:/var/run/docker.sock:ro
    restart: unless-stopped`,
    binarySnippet: `# /etc/otelcol/config.yaml
receivers:
  docker_stats:
    endpoint: unix:///var/run/docker.sock
    collection_interval: 30s

exporters:${exporterBlock("metrics")}

service:
  pipelines:
    metrics:
      receivers: [docker_stats]
      exporters: [otlphttp/metrics]`,
  },

  {
    slug: "mysql",
    name: "MySQL",
    description: "Collect MySQL performance metrics and query stats",
    icon: "Database",
    category: "metrics",
    dataType: "Metrics",
    setupNotes: "Deploy the OpenTelemetry Collector with the MySQL receiver. It connects to MySQL via TCP and collects query performance, connection pool, buffer pool, table lock, and InnoDB metrics. Create a dedicated monitoring user with SELECT and PROCESS privileges.",
    prerequisites: ["MySQL 5.7+ or 8.0+", "Monitoring user with SELECT, PROCESS privileges"],
    verifyPath: "/v1/metrics",
    dockerSnippet: `services:
  otelcol:
    image: ${OTELCOL_IMAGE}
    volumes:
      - ./otelcol-mysql.yaml:/etc/otelcol-contrib/config.yaml
    environment:
      MYSQL_USER: otel
      MYSQL_PASSWORD: your_password
    restart: unless-stopped`,
    binarySnippet: `# /etc/otelcol/config.yaml
receivers:
  mysql:
    endpoint: localhost:3306
    username: otel
    password: your_password
    collection_interval: 30s

exporters:${exporterBlock("metrics")}

service:
  pipelines:
    metrics:
      receivers: [mysql]
      exporters: [otlphttp/metrics]`,
  },

  {
    slug: "postgresql",
    name: "PostgreSQL",
    description: "Collect PostgreSQL performance metrics and query stats",
    icon: "Database",
    category: "metrics",
    dataType: "Metrics",
    setupNotes: "Deploy the OpenTelemetry Collector with the PostgreSQL receiver. It connects to PostgreSQL and collects database size, active connections, transaction throughput, index usage, and replication lag. Create a dedicated monitoring user with pg_monitor role.",
    prerequisites: ["PostgreSQL 12+", "Monitoring user with pg_monitor role"],
    verifyPath: "/v1/metrics",
    dockerSnippet: `services:
  otelcol:
    image: ${OTELCOL_IMAGE}
    volumes:
      - ./otelcol-pg.yaml:/etc/otelcol-contrib/config.yaml
    restart: unless-stopped`,
    binarySnippet: `# /etc/otelcol/config.yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    username: otel
    password: your_password
    databases: [mydb]
    collection_interval: 30s

exporters:${exporterBlock("metrics")}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      exporters: [otlphttp/metrics]`,
  },

  // ── Logs ─────────────────────────────────────────────────────────────────

  {
    slug: "filelogs",
    name: "File Logs",
    description: "Tail log files and ship them to the ingestion pipeline",
    icon: "FileText",
    category: "logs",
    dataType: "Logs",
    setupNotes: "Deploy the OpenTelemetry Collector with the filelog receiver. It tails specified log files and streams new lines as log records. Supports glob patterns for multi-file matching, multiline log parsing, and custom operators for JSON/regex parsing.",
    prerequisites: ["Read access to log files"],
    verifyPath: "/v1/logs",
    dockerSnippet: `services:
  otelcol:
    image: ${OTELCOL_IMAGE}
    volumes:
      - ./otelcol-filelog.yaml:/etc/otelcol-contrib/config.yaml
      - /var/log:/var/log:ro
    restart: unless-stopped`,
    binarySnippet: `# /etc/otelcol/config.yaml
receivers:
  filelog:
    include: [/var/log/app/*.log]
    start_at: end

exporters:${exporterBlock("logs")}

service:
  pipelines:
    logs:
      receivers: [filelog]
      exporters: [otlphttp/logs]`,
  },

  {
    slug: "dockerlogs",
    name: "Docker Logs",
    description: "Collect stdout/stderr logs from all Docker containers",
    icon: "ScrollText",
    category: "logs",
    dataType: "Logs",
    setupNotes: "Deploy the OpenTelemetry Collector with the filelog receiver pointed at Docker's JSON log driver output. It reads container log files, parses the JSON envelope, and extracts the actual log body. Container metadata is automatically added as attributes.",
    prerequisites: ["Docker Engine with json-file log driver (default)", "Read access to /var/lib/docker/containers"],
    verifyPath: "/v1/logs",
    dockerSnippet: `services:
  otelcol:
    image: ${OTELCOL_IMAGE}
    volumes:
      - ./otelcol-dockerlogs.yaml:/etc/otelcol-contrib/config.yaml
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - /var/lib/docker/containers:/var/lib/docker/containers:ro
    restart: unless-stopped`,
    binarySnippet: `# /etc/otelcol/config.yaml
receivers:
  filelog:
    include: [/var/lib/docker/containers/*/*-json.log]
    operators:
      - type: json_parser
      - type: move
        from: attributes.log
        to: body

exporters:${exporterBlock("logs")}

service:
  pipelines:
    logs:
      receivers: [filelog]
      exporters: [otlphttp/logs]`,
  },

  {
    slug: "nginx",
    name: "Nginx",
    description: "Collect Nginx access logs and metrics",
    icon: "Globe",
    category: "logs",
    dataType: "Logs",
    setupNotes: "Deploy the OpenTelemetry Collector with the filelog receiver pointed at Nginx log files. It tails both access.log and error.log. For structured parsing, configure the combined log format in Nginx and add a regex_parser operator in the collector config.",
    prerequisites: ["Nginx with file-based logging enabled", "Read access to /var/log/nginx/"],
    verifyPath: "/v1/logs",
    dockerSnippet: `services:
  otelcol:
    image: ${OTELCOL_IMAGE}
    volumes:
      - ./otelcol-nginx.yaml:/etc/otelcol-contrib/config.yaml
      - /var/log/nginx:/var/log/nginx:ro
    restart: unless-stopped`,
    binarySnippet: `# /etc/otelcol/config.yaml
receivers:
  filelog:
    include: [/var/log/nginx/access.log, /var/log/nginx/error.log]
    start_at: end

exporters:${exporterBlock("logs")}

service:
  pipelines:
    logs:
      receivers: [filelog]
      exporters: [otlphttp/logs]`,
  },
]
