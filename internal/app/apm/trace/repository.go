package trace

import (
	"context"
	"fmt"
	. "metis/internal/app/apm/clickhouse"
	"strings"
	"time"

	"github.com/samber/do/v2"
)

// TraceSummary is the list-level representation of a trace.
type TraceSummary struct {
	TraceId       string    `json:"traceId"`
	ServiceName   string    `json:"serviceName"`
	RootOperation string    `json:"rootOperation"`
	DurationMs    float64   `json:"durationMs"`
	StatusCode    string    `json:"statusCode"`
	Timestamp     time.Time `json:"timestamp"`
	SpanCount     uint64    `json:"spanCount"`
	HasError      bool      `json:"hasError"`
}

// Span is a single span within a trace.
type Span struct {
	TraceId            string            `json:"traceId"`
	SpanId             string            `json:"spanId"`
	ParentSpanId       string            `json:"parentSpanId"`
	ServiceName        string            `json:"serviceName"`
	SpanName           string            `json:"spanName"`
	SpanKind           string            `json:"spanKind"`
	StartTime          time.Time         `json:"startTime"`
	Duration           int64             `json:"duration"` // nanoseconds
	StatusCode         string            `json:"statusCode"`
	StatusMessage      string            `json:"statusMessage"`
	SpanAttributes     map[string]string `json:"spanAttributes"`
	ResourceAttributes map[string]string `json:"resourceAttributes"`
	Events             []SpanEvent       `json:"events"`
}

// SpanEvent is an event attached to a span.
type SpanEvent struct {
	Timestamp  time.Time         `json:"timestamp"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"`
}

// ServiceOverview is the list-level representation of a service in the catalog.
type ServiceOverview struct {
	ServiceName  string    `json:"serviceName"`
	RequestCount uint64    `json:"requestCount"`
	AvgDuration  float64   `json:"avgDurationMs"`
	P50          float64   `json:"p50Ms"`
	P95          float64   `json:"p95Ms"`
	P99          float64   `json:"p99Ms"`
	ErrorRate    float64   `json:"errorRate"`
	FirstSeen    time.Time `json:"firstSeen"`
	LastSeen     time.Time `json:"lastSeen"`
}

// ServiceDetail holds aggregated metrics for a single service.
type ServiceDetail struct {
	ServiceName  string           `json:"serviceName"`
	RequestCount uint64           `json:"requestCount"`
	AvgDuration  float64          `json:"avgDurationMs"`
	P50          float64          `json:"p50Ms"`
	P95          float64          `json:"p95Ms"`
	P99          float64          `json:"p99Ms"`
	ErrorRate    float64          `json:"errorRate"`
	Operations   []OperationStats `json:"operations"`
}

// OperationStats holds per-operation metrics for a service.
type OperationStats struct {
	SpanName     string  `json:"spanName"`
	RequestCount uint64  `json:"requestCount"`
	AvgDuration  float64 `json:"avgDurationMs"`
	P50          float64 `json:"p50Ms"`
	P95          float64 `json:"p95Ms"`
	P99          float64 `json:"p99Ms"`
	ErrorRate    float64 `json:"errorRate"`
}

// TimeseriesPoint is a single data point in a timeseries aggregation.
type TimeseriesPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	RequestCount uint64    `json:"requestCount"`
	AvgDuration  float64   `json:"avgDurationMs"`
	P50          float64   `json:"p50Ms"`
	P95          float64   `json:"p95Ms"`
	P99          float64   `json:"p99Ms"`
	ErrorRate    float64   `json:"errorRate"`
}

// SparklinePoint is a minimal data point for inline sparklines.
type SparklinePoint struct {
	Timestamp    time.Time `json:"timestamp"`
	RequestCount uint64    `json:"requestCount"`
	ErrorRate    float64   `json:"errorRate"`
	AvgDuration  float64   `json:"avgDurationMs"`
}

// TimeseriesParams holds query parameters for timeseries aggregation.
type TimeseriesParams struct {
	Start     time.Time
	End       time.Time
	Service   string
	Operation string
	Interval  int // seconds
}

// TraceFilters holds query parameters for listing traces.
type TraceFilters struct {
	Start       time.Time
	End         time.Time
	Service     string
	Operation   string
	Status      string // "ok", "error", or ""
	DurationMin *float64
	DurationMax *float64
	Page        int
	PageSize    int
}

// Repository handles ClickHouse queries for APM data.
type Repository struct {
	ch *ClickHouseClient
}

// NewRepository creates a new APM repository.
func NewRepository(i do.Injector) (*Repository, error) {
	ch := do.MustInvoke[*ClickHouseClient](i)
	return &Repository{ch: ch}, nil
}

// ListTraces returns trace summaries matching the given filters.
func (r *Repository) ListTraces(ctx context.Context, f TraceFilters) ([]TraceSummary, int64, error) {
	if r.ch == nil {
		return nil, 0, nil
	}

	var conditions []string
	var args []any

	conditions = append(conditions, "t.Timestamp >= ?", "t.Timestamp <= ?")
	args = append(args, f.Start, f.End)

	conditions = append(conditions, "t.ParentSpanId = ''")

	if f.Service != "" {
		conditions = append(conditions, "t.ServiceName = ?")
		args = append(args, f.Service)
	}
	if f.Operation != "" {
		conditions = append(conditions, "t.SpanName = ?")
		args = append(args, f.Operation)
	}
	if f.Status == "ok" {
		conditions = append(conditions, "t.StatusCode != 'STATUS_CODE_ERROR'")
	} else if f.Status == "error" {
		conditions = append(conditions, "t.StatusCode = 'STATUS_CODE_ERROR'")
	}
	if f.DurationMin != nil {
		conditions = append(conditions, "t.Duration / 1e6 >= ?")
		args = append(args, *f.DurationMin)
	}
	if f.DurationMax != nil {
		conditions = append(conditions, "t.Duration / 1e6 <= ?")
		args = append(args, *f.DurationMax)
	}

	where := strings.Join(conditions, " AND ")

	// Count query
	countSQL := fmt.Sprintf(`SELECT count() FROM otel_traces t WHERE %s`, where)
	var total int64
	if err := r.ch.DB.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count traces: %w", err)
	}

	if total == 0 {
		return []TraceSummary{}, 0, nil
	}

	// Data query with subquery for span counts
	offset := (f.Page - 1) * f.PageSize
	dataSQL := fmt.Sprintf(`
		SELECT
			t.TraceId,
			t.ServiceName,
			t.SpanName AS RootOperation,
			t.Duration / 1e6 AS DurationMs,
			t.StatusCode,
			t.Timestamp,
			counts.SpanCount,
			counts.HasError
		FROM otel_traces t
		INNER JOIN (
			SELECT
				TraceId,
				count() AS SpanCount,
				max(StatusCode = 'STATUS_CODE_ERROR') AS HasError
			FROM otel_traces
			WHERE Timestamp >= ? AND Timestamp <= ?
			GROUP BY TraceId
		) counts ON t.TraceId = counts.TraceId
		WHERE %s
		ORDER BY t.Timestamp DESC
		LIMIT ? OFFSET ?
	`, where)

	// Prepend subquery time args, then main where args, then limit/offset
	queryArgs := []any{f.Start, f.End}
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, f.PageSize, offset)

	rows, err := r.ch.DB.QueryContext(ctx, dataSQL, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list traces: %w", err)
	}
	defer rows.Close()

	var traces []TraceSummary
	for rows.Next() {
		var t TraceSummary
		var hasErr uint8
		if err := rows.Scan(&t.TraceId, &t.ServiceName, &t.RootOperation, &t.DurationMs, &t.StatusCode, &t.Timestamp, &t.SpanCount, &hasErr); err != nil {
			return nil, 0, fmt.Errorf("scan trace: %w", err)
		}
		t.HasError = hasErr > 0
		traces = append(traces, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate traces: %w", err)
	}

	return traces, total, nil
}

// GetTrace returns all spans for a given trace ID.
func (r *Repository) GetTrace(ctx context.Context, traceId string) ([]Span, error) {
	if r.ch == nil {
		return nil, nil
	}

	query := `
		SELECT
			TraceId, SpanId, ParentSpanId, ServiceName, SpanName, SpanKind,
			Timestamp, Duration, StatusCode, StatusMessage,
			SpanAttributes, ResourceAttributes,
			Events.Timestamp, Events.Name, Events.Attributes
		FROM otel_traces
		WHERE TraceId = ?
		ORDER BY Timestamp ASC
	`

	rows, err := r.ch.DB.QueryContext(ctx, query, traceId)
	if err != nil {
		return nil, fmt.Errorf("get trace %s: %w", traceId, err)
	}
	defer rows.Close()

	var spans []Span
	for rows.Next() {
		var s Span
		var eventTimestamps []time.Time
		var eventNames []string
		var eventAttrs []map[string]string

		if err := rows.Scan(
			&s.TraceId, &s.SpanId, &s.ParentSpanId, &s.ServiceName, &s.SpanName, &s.SpanKind,
			&s.StartTime, &s.Duration, &s.StatusCode, &s.StatusMessage,
			&s.SpanAttributes, &s.ResourceAttributes,
			&eventTimestamps, &eventNames, &eventAttrs,
		); err != nil {
			return nil, fmt.Errorf("scan span: %w", err)
		}

		// Zip event arrays into SpanEvent structs
		for i := range eventTimestamps {
			evt := SpanEvent{Timestamp: eventTimestamps[i]}
			if i < len(eventNames) {
				evt.Name = eventNames[i]
			}
			if i < len(eventAttrs) {
				evt.Attributes = eventAttrs[i]
			}
			s.Events = append(s.Events, evt)
		}

		spans = append(spans, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate spans: %w", err)
	}

	return spans, nil
}

// ListServices returns aggregated metrics for all services within the time range.
func (r *Repository) ListServices(ctx context.Context, start, end time.Time) ([]ServiceOverview, error) {
	if r.ch == nil {
		return nil, nil
	}

	query := `
		SELECT
			ServiceName,
			count()                                                      AS RequestCount,
			avg(Duration) / 1e6                                          AS AvgDurationMs,
			quantile(0.50)(Duration) / 1e6                               AS P50Ms,
			quantile(0.95)(Duration) / 1e6                               AS P95Ms,
			quantile(0.99)(Duration) / 1e6                               AS P99Ms,
			countIf(StatusCode = 'STATUS_CODE_ERROR') * 100.0 / count()  AS ErrorRate,
			min(Timestamp)                                               AS FirstSeen,
			max(Timestamp)                                               AS LastSeen
		FROM otel_traces
		WHERE Timestamp >= ? AND Timestamp <= ?
		  AND SpanKind = 'SPAN_KIND_SERVER'
		GROUP BY ServiceName
		ORDER BY RequestCount DESC
	`

	rows, err := r.ch.DB.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	var services []ServiceOverview
	for rows.Next() {
		var s ServiceOverview
		if err := rows.Scan(&s.ServiceName, &s.RequestCount, &s.AvgDuration, &s.P50, &s.P95, &s.P99, &s.ErrorRate, &s.FirstSeen, &s.LastSeen); err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		services = append(services, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate services: %w", err)
	}

	return services, nil
}

// GetServiceDetail returns aggregated metrics for a single service.
func (r *Repository) GetServiceDetail(ctx context.Context, serviceName string, start, end time.Time) (*ServiceDetail, error) {
	if r.ch == nil {
		return nil, nil
	}

	// Overall metrics
	overallQuery := `
		SELECT
			count()                                                      AS RequestCount,
			avg(Duration) / 1e6                                          AS AvgDurationMs,
			quantile(0.50)(Duration) / 1e6                               AS P50Ms,
			quantile(0.95)(Duration) / 1e6                               AS P95Ms,
			quantile(0.99)(Duration) / 1e6                               AS P99Ms,
			countIf(StatusCode = 'STATUS_CODE_ERROR') * 100.0 / count()  AS ErrorRate
		FROM otel_traces
		WHERE Timestamp >= ? AND Timestamp <= ?
		  AND ServiceName = ?
		  AND SpanKind = 'SPAN_KIND_SERVER'
	`

	detail := &ServiceDetail{ServiceName: serviceName}
	if err := r.ch.DB.QueryRowContext(ctx, overallQuery, start, end, serviceName).Scan(
		&detail.RequestCount, &detail.AvgDuration, &detail.P50, &detail.P95, &detail.P99, &detail.ErrorRate,
	); err != nil {
		return nil, fmt.Errorf("get service detail: %w", err)
	}

	// Operations
	ops, err := r.ListOperations(ctx, serviceName, start, end)
	if err != nil {
		return nil, err
	}
	detail.Operations = ops

	return detail, nil
}

// ListOperations returns per-operation metrics for a service.
func (r *Repository) ListOperations(ctx context.Context, serviceName string, start, end time.Time) ([]OperationStats, error) {
	if r.ch == nil {
		return nil, nil
	}

	query := `
		SELECT
			SpanName,
			count()                                                      AS RequestCount,
			avg(Duration) / 1e6                                          AS AvgDurationMs,
			quantile(0.50)(Duration) / 1e6                               AS P50Ms,
			quantile(0.95)(Duration) / 1e6                               AS P95Ms,
			quantile(0.99)(Duration) / 1e6                               AS P99Ms,
			countIf(StatusCode = 'STATUS_CODE_ERROR') * 100.0 / count()  AS ErrorRate
		FROM otel_traces
		WHERE Timestamp >= ? AND Timestamp <= ?
		  AND ServiceName = ?
		  AND SpanKind = 'SPAN_KIND_SERVER'
		GROUP BY SpanName
		ORDER BY RequestCount DESC
	`

	rows, err := r.ch.DB.QueryContext(ctx, query, start, end, serviceName)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	defer rows.Close()

	var ops []OperationStats
	for rows.Next() {
		var o OperationStats
		if err := rows.Scan(&o.SpanName, &o.RequestCount, &o.AvgDuration, &o.P50, &o.P95, &o.P99, &o.ErrorRate); err != nil {
			return nil, fmt.Errorf("scan operation: %w", err)
		}
		ops = append(ops, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate operations: %w", err)
	}

	return ops, nil
}

// GetTimeseries returns time-bucketed aggregation for a service/operation.
func (r *Repository) GetTimeseries(ctx context.Context, p TimeseriesParams) ([]TimeseriesPoint, error) {
	if r.ch == nil {
		return nil, nil
	}

	var conditions []string
	var args []any

	conditions = append(conditions, "Timestamp >= ?", "Timestamp <= ?")
	args = append(args, p.Start, p.End)
	conditions = append(conditions, "SpanKind = 'SPAN_KIND_SERVER'")

	if p.Service != "" {
		conditions = append(conditions, "ServiceName = ?")
		args = append(args, p.Service)
	}
	if p.Operation != "" {
		conditions = append(conditions, "SpanName = ?")
		args = append(args, p.Operation)
	}

	where := strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT
			toStartOfInterval(Timestamp, INTERVAL %d SECOND)            AS ts,
			count()                                                      AS RequestCount,
			avg(Duration) / 1e6                                          AS AvgDurationMs,
			quantile(0.50)(Duration) / 1e6                               AS P50Ms,
			quantile(0.95)(Duration) / 1e6                               AS P95Ms,
			quantile(0.99)(Duration) / 1e6                               AS P99Ms,
			countIf(StatusCode = 'STATUS_CODE_ERROR') * 100.0 / count()  AS ErrorRate
		FROM otel_traces
		WHERE %s
		GROUP BY ts
		ORDER BY ts ASC
	`, p.Interval, where)

	rows, err := r.ch.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get timeseries: %w", err)
	}
	defer rows.Close()

	var points []TimeseriesPoint
	for rows.Next() {
		var pt TimeseriesPoint
		if err := rows.Scan(&pt.Timestamp, &pt.RequestCount, &pt.AvgDuration, &pt.P50, &pt.P95, &pt.P99, &pt.ErrorRate); err != nil {
			return nil, fmt.Errorf("scan timeseries: %w", err)
		}
		points = append(points, pt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate timeseries: %w", err)
	}

	return points, nil
}

// GetServiceSparklines returns mini timeseries for all services (for the catalog sparkline).
func (r *Repository) GetServiceSparklines(ctx context.Context, start, end time.Time, interval int) (map[string][]SparklinePoint, error) {
	if r.ch == nil {
		return nil, nil
	}

	query := fmt.Sprintf(`
		SELECT
			ServiceName,
			toStartOfInterval(Timestamp, INTERVAL %d SECOND)            AS ts,
			count()                                                      AS RequestCount,
			countIf(StatusCode = 'STATUS_CODE_ERROR') * 100.0 / count()  AS ErrorRate,
			avg(Duration) / 1e6                                          AS AvgDurationMs
		FROM otel_traces
		WHERE Timestamp >= ? AND Timestamp <= ?
		  AND SpanKind = 'SPAN_KIND_SERVER'
		GROUP BY ServiceName, ts
		ORDER BY ServiceName, ts ASC
	`, interval)

	rows, err := r.ch.DB.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get sparklines: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]SparklinePoint)
	for rows.Next() {
		var svc string
		var pt SparklinePoint
		if err := rows.Scan(&svc, &pt.Timestamp, &pt.RequestCount, &pt.ErrorRate, &pt.AvgDuration); err != nil {
			return nil, fmt.Errorf("scan sparkline: %w", err)
		}
		result[svc] = append(result[svc], pt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sparklines: %w", err)
	}

	return result, nil
}

// TopologyNode represents a service node in the topology graph.
type TopologyNode struct {
	ServiceName  string  `json:"serviceName"`
	RequestCount uint64  `json:"requestCount"`
	ErrorRate    float64 `json:"errorRate"`
}

// TopologyEdge represents a caller→callee relationship.
type TopologyEdge struct {
	Caller      string  `json:"caller"`
	Callee      string  `json:"callee"`
	CallCount   uint64  `json:"callCount"`
	AvgDuration float64 `json:"avgDurationMs"`
	P95Duration float64 `json:"p95DurationMs"`
	ErrorRate   float64 `json:"errorRate"`
}

// TopologyGraph holds the full service topology.
type TopologyGraph struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

// TraceLog is a log record correlated with a trace.
type TraceLog struct {
	Timestamp          time.Time         `json:"timestamp"`
	SeverityText       string            `json:"severityText"`
	SeverityNumber     int32             `json:"severityNumber"`
	Body               string            `json:"body"`
	ServiceName        string            `json:"serviceName"`
	TraceId            string            `json:"traceId"`
	SpanId             string            `json:"spanId"`
	LogAttributes      map[string]string `json:"logAttributes"`
	ResourceAttributes map[string]string `json:"resourceAttributes"`
}

// GetTopology returns the service call topology graph for the given time range.
func (r *Repository) GetTopology(ctx context.Context, start, end time.Time) (*TopologyGraph, error) {
	if r.ch == nil {
		return nil, nil
	}

	// 1. Get edges via self-JOIN on cross-service parent-child spans
	edgeQuery := `
		SELECT
			parent.ServiceName                                            AS Caller,
			child.ServiceName                                             AS Callee,
			count()                                                       AS CallCount,
			avg(child.Duration) / 1e6                                     AS AvgDurationMs,
			quantile(0.95)(child.Duration) / 1e6                          AS P95DurationMs,
			countIf(child.StatusCode = 'STATUS_CODE_ERROR') * 100.0 / count() AS ErrorRate
		FROM otel_traces AS child
		INNER JOIN otel_traces AS parent
			ON child.ParentSpanId = parent.SpanId
			AND child.TraceId = parent.TraceId
		WHERE child.ServiceName != parent.ServiceName
		  AND child.Timestamp >= ? AND child.Timestamp <= ?
		  AND parent.Timestamp >= ? AND parent.Timestamp <= ?
		GROUP BY Caller, Callee
	`

	rows, err := r.ch.DB.QueryContext(ctx, edgeQuery, start, end, start, end)
	if err != nil {
		return nil, fmt.Errorf("get topology edges: %w", err)
	}
	defer rows.Close()

	var edges []TopologyEdge
	serviceSet := make(map[string]struct{})
	for rows.Next() {
		var e TopologyEdge
		if err := rows.Scan(&e.Caller, &e.Callee, &e.CallCount, &e.AvgDuration, &e.P95Duration, &e.ErrorRate); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		edges = append(edges, e)
		serviceSet[e.Caller] = struct{}{}
		serviceSet[e.Callee] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate edges: %w", err)
	}

	// 2. Get per-node metrics for all services seen in edges
	nodeQuery := `
		SELECT
			ServiceName,
			count()                                                       AS RequestCount,
			countIf(StatusCode = 'STATUS_CODE_ERROR') * 100.0 / count()   AS ErrorRate
		FROM otel_traces
		WHERE Timestamp >= ? AND Timestamp <= ?
		  AND SpanKind = 'SPAN_KIND_SERVER'
		GROUP BY ServiceName
	`
	nodeRows, err := r.ch.DB.QueryContext(ctx, nodeQuery, start, end)
	if err != nil {
		return nil, fmt.Errorf("get topology nodes: %w", err)
	}
	defer nodeRows.Close()

	var nodes []TopologyNode
	for nodeRows.Next() {
		var n TopologyNode
		if err := nodeRows.Scan(&n.ServiceName, &n.RequestCount, &n.ErrorRate); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		nodes = append(nodes, n)
		serviceSet[n.ServiceName] = struct{}{}
	}
	if err := nodeRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nodes: %w", err)
	}

	// Ensure all services from edges have a node entry
	existingNodes := make(map[string]bool)
	for _, n := range nodes {
		existingNodes[n.ServiceName] = true
	}
	for svc := range serviceSet {
		if !existingNodes[svc] {
			nodes = append(nodes, TopologyNode{ServiceName: svc})
		}
	}

	return &TopologyGraph{Nodes: nodes, Edges: edges}, nil
}

// GetLogsByTraceId returns log records correlated with a trace ID.
// The bool return indicates whether otel_logs table is available.
func (r *Repository) GetLogsByTraceId(ctx context.Context, traceId string) ([]TraceLog, bool, error) {
	if r.ch == nil {
		return nil, false, nil
	}

	query := `
		SELECT
			Timestamp, SeverityText, SeverityNumber, Body,
			ServiceName, TraceId, SpanId,
			LogAttributes, ResourceAttributes
		FROM otel_logs
		WHERE TraceId = ?
		ORDER BY Timestamp ASC
	`

	rows, err := r.ch.DB.QueryContext(ctx, query, traceId)
	if err != nil {
		// Check if the error is about table not existing
		errMsg := err.Error()
		if strings.Contains(errMsg, "doesn't exist") || strings.Contains(errMsg, "UNKNOWN_TABLE") {
			return nil, false, nil
		}
		return nil, true, fmt.Errorf("get trace logs: %w", err)
	}
	defer rows.Close()

	var logs []TraceLog
	for rows.Next() {
		var l TraceLog
		if err := rows.Scan(
			&l.Timestamp, &l.SeverityText, &l.SeverityNumber, &l.Body,
			&l.ServiceName, &l.TraceId, &l.SpanId,
			&l.LogAttributes, &l.ResourceAttributes,
		); err != nil {
			return nil, true, fmt.Errorf("scan log: %w", err)
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, true, fmt.Errorf("iterate logs: %w", err)
	}

	return logs, true, nil
}

// --- Analytics APIs ---

// SpanSearchParams holds parameters for span attribute search.
type SpanSearchParams struct {
	Start time.Time
	End   time.Time
	Key   string
	Value string
	Op    string // "eq", "contains", "exists"
	Page  int
	Size  int
}

// SearchSpans searches spans by attribute key/value.
func (r *Repository) SearchSpans(ctx context.Context, p SpanSearchParams) ([]Span, int64, error) {
	if r.ch == nil {
		return nil, 0, nil
	}

	var conditions []string
	var args []any

	conditions = append(conditions, "Timestamp >= ?", "Timestamp <= ?")
	args = append(args, p.Start, p.End)

	switch p.Op {
	case "contains":
		conditions = append(conditions, fmt.Sprintf("SpanAttributes['%s'] LIKE ?", p.Key))
		args = append(args, "%"+p.Value+"%")
	case "exists":
		conditions = append(conditions, fmt.Sprintf("mapContains(SpanAttributes, '%s')", p.Key))
	default: // eq
		conditions = append(conditions, fmt.Sprintf("SpanAttributes['%s'] = ?", p.Key))
		args = append(args, p.Value)
	}

	where := strings.Join(conditions, " AND ")

	countSQL := fmt.Sprintf(`SELECT count() FROM otel_traces WHERE %s`, where)
	var total int64
	if err := r.ch.DB.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count span search: %w", err)
	}
	if total == 0 {
		return []Span{}, 0, nil
	}

	offset := (p.Page - 1) * p.Size
	dataSQL := fmt.Sprintf(`
		SELECT
			TraceId, SpanId, ParentSpanId, ServiceName, SpanName, SpanKind,
			Timestamp, Duration, StatusCode, StatusMessage,
			SpanAttributes, ResourceAttributes,
			Events.Timestamp, Events.Name, Events.Attributes
		FROM otel_traces
		WHERE %s
		ORDER BY Timestamp DESC
		LIMIT ? OFFSET ?
	`, where)

	queryArgs := append(args, p.Size, offset)
	rows, err := r.ch.DB.QueryContext(ctx, dataSQL, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("search spans: %w", err)
	}
	defer rows.Close()

	var spans []Span
	for rows.Next() {
		var s Span
		var eventTimestamps []time.Time
		var eventNames []string
		var eventAttrs []map[string]string
		if err := rows.Scan(
			&s.TraceId, &s.SpanId, &s.ParentSpanId, &s.ServiceName, &s.SpanName, &s.SpanKind,
			&s.StartTime, &s.Duration, &s.StatusCode, &s.StatusMessage,
			&s.SpanAttributes, &s.ResourceAttributes,
			&eventTimestamps, &eventNames, &eventAttrs,
		); err != nil {
			return nil, 0, fmt.Errorf("scan span search: %w", err)
		}
		for i := range eventTimestamps {
			evt := SpanEvent{Timestamp: eventTimestamps[i]}
			if i < len(eventNames) {
				evt.Name = eventNames[i]
			}
			if i < len(eventAttrs) {
				evt.Attributes = eventAttrs[i]
			}
			s.Events = append(s.Events, evt)
		}
		spans = append(spans, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate span search: %w", err)
	}

	return spans, total, nil
}

// AnalyticsParams holds parameters for aggregation analytics.
type AnalyticsParams struct {
	Start     time.Time
	End       time.Time
	GroupBy   string // "service", "operation", "statusCode"
	Service   string
	Operation string
}

// AnalyticsGroup is one row from a group-by aggregation.
type AnalyticsGroup struct {
	Key          string  `json:"key"`
	RequestCount uint64  `json:"requestCount"`
	AvgDuration  float64 `json:"avgDurationMs"`
	P95          float64 `json:"p95Ms"`
	ErrorRate    float64 `json:"errorRate"`
}

// GetAnalytics returns aggregated metrics grouped by the specified dimension.
func (r *Repository) GetAnalytics(ctx context.Context, p AnalyticsParams) ([]AnalyticsGroup, error) {
	if r.ch == nil {
		return nil, nil
	}

	var groupCol string
	switch p.GroupBy {
	case "operation":
		groupCol = "SpanName"
	case "statusCode":
		groupCol = "StatusCode"
	default:
		groupCol = "ServiceName"
	}

	var conditions []string
	var args []any
	conditions = append(conditions, "Timestamp >= ?", "Timestamp <= ?")
	args = append(args, p.Start, p.End)
	conditions = append(conditions, "SpanKind = 'SPAN_KIND_SERVER'")

	if p.Service != "" {
		conditions = append(conditions, "ServiceName = ?")
		args = append(args, p.Service)
	}
	if p.Operation != "" {
		conditions = append(conditions, "SpanName = ?")
		args = append(args, p.Operation)
	}

	where := strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT
			%s AS GroupKey,
			count() AS RequestCount,
			avg(Duration) / 1e6 AS AvgDurationMs,
			quantile(0.95)(Duration) / 1e6 AS P95Ms,
			countIf(StatusCode = 'STATUS_CODE_ERROR') * 100.0 / count() AS ErrorRate
		FROM otel_traces
		WHERE %s
		GROUP BY GroupKey
		ORDER BY RequestCount DESC
	`, groupCol, where)

	rows, err := r.ch.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get analytics: %w", err)
	}
	defer rows.Close()

	var groups []AnalyticsGroup
	for rows.Next() {
		var g AnalyticsGroup
		if err := rows.Scan(&g.Key, &g.RequestCount, &g.AvgDuration, &g.P95, &g.ErrorRate); err != nil {
			return nil, fmt.Errorf("scan analytics: %w", err)
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate analytics: %w", err)
	}

	return groups, nil
}

// LatencyDistParams holds parameters for latency distribution query.
type LatencyDistParams struct {
	Start     time.Time
	End       time.Time
	Service   string
	Operation string
	Buckets   int
}

// LatencyBucket is one bucket of the latency distribution histogram.
type LatencyBucket struct {
	RangeStartMs float64 `json:"rangeStartMs"`
	RangeEndMs   float64 `json:"rangeEndMs"`
	Count        uint64  `json:"count"`
}

// GetLatencyDistribution returns a histogram of latency distribution.
func (r *Repository) GetLatencyDistribution(ctx context.Context, p LatencyDistParams) ([]LatencyBucket, error) {
	if r.ch == nil {
		return nil, nil
	}

	if p.Buckets <= 0 {
		p.Buckets = 20
	}

	var conditions []string
	var args []any
	conditions = append(conditions, "Timestamp >= ?", "Timestamp <= ?")
	args = append(args, p.Start, p.End)
	conditions = append(conditions, "SpanKind = 'SPAN_KIND_SERVER'")

	if p.Service != "" {
		conditions = append(conditions, "ServiceName = ?")
		args = append(args, p.Service)
	}
	if p.Operation != "" {
		conditions = append(conditions, "SpanName = ?")
		args = append(args, p.Operation)
	}

	where := strings.Join(conditions, " AND ")

	// First get min/max to compute bucket boundaries
	rangeSQL := fmt.Sprintf(`SELECT min(Duration/1e6), max(Duration/1e6) FROM otel_traces WHERE %s`, where)
	var minMs, maxMs float64
	if err := r.ch.DB.QueryRowContext(ctx, rangeSQL, args...).Scan(&minMs, &maxMs); err != nil {
		return nil, fmt.Errorf("get latency range: %w", err)
	}
	if maxMs <= minMs {
		return []LatencyBucket{}, nil
	}

	bucketWidth := (maxMs - minMs) / float64(p.Buckets)

	query := fmt.Sprintf(`
		SELECT
			floor((Duration/1e6 - ?) / ?) AS BucketIdx,
			count() AS Count
		FROM otel_traces
		WHERE %s
		GROUP BY BucketIdx
		ORDER BY BucketIdx ASC
	`, where)

	queryArgs := []any{minMs, bucketWidth}
	queryArgs = append(queryArgs, args...)

	rows, err := r.ch.DB.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("get latency distribution: %w", err)
	}
	defer rows.Close()

	bucketMap := make(map[int]uint64)
	for rows.Next() {
		var idx int
		var count uint64
		if err := rows.Scan(&idx, &count); err != nil {
			return nil, fmt.Errorf("scan latency bucket: %w", err)
		}
		if idx >= p.Buckets {
			idx = p.Buckets - 1
		}
		if idx < 0 {
			idx = 0
		}
		bucketMap[idx] += count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latency buckets: %w", err)
	}

	buckets := make([]LatencyBucket, p.Buckets)
	for i := range buckets {
		buckets[i] = LatencyBucket{
			RangeStartMs: minMs + float64(i)*bucketWidth,
			RangeEndMs:   minMs + float64(i+1)*bucketWidth,
			Count:        bucketMap[i],
		}
	}

	return buckets, nil
}

// ErrorGroup is one aggregated error group.
type ErrorGroup struct {
	ErrorType string    `json:"errorType"`
	Message   string    `json:"message"`
	Count     uint64    `json:"count"`
	LastSeen  time.Time `json:"lastSeen"`
	Services  []string  `json:"services"`
}

// GetErrors returns error groups aggregated by error type and message.
func (r *Repository) GetErrors(ctx context.Context, start, end time.Time, service string) ([]ErrorGroup, error) {
	if r.ch == nil {
		return nil, nil
	}

	var conditions []string
	var args []any
	conditions = append(conditions, "Timestamp >= ?", "Timestamp <= ?")
	args = append(args, start, end)
	conditions = append(conditions, "StatusCode = 'STATUS_CODE_ERROR'")

	if service != "" {
		conditions = append(conditions, "ServiceName = ?")
		args = append(args, service)
	}

	where := strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT
			SpanAttributes['exception.type'] AS ErrorType,
			StatusMessage AS Message,
			count() AS Count,
			max(Timestamp) AS LastSeen,
			groupUniqArray(ServiceName) AS Services
		FROM otel_traces
		WHERE %s
		GROUP BY ErrorType, Message
		ORDER BY Count DESC
		LIMIT 100
	`, where)

	rows, err := r.ch.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get errors: %w", err)
	}
	defer rows.Close()

	var groups []ErrorGroup
	for rows.Next() {
		var g ErrorGroup
		if err := rows.Scan(&g.ErrorType, &g.Message, &g.Count, &g.LastSeen, &g.Services); err != nil {
			return nil, fmt.Errorf("scan error group: %w", err)
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate errors: %w", err)
	}

	return groups, nil
}
