package trace

import (
	"context"
	"errors"
	. "metis/internal/app/apm/clickhouse"
	"time"

	"github.com/samber/do/v2"
)

// ErrClickHouseNotConfigured is returned when ClickHouse is not configured.
var ErrClickHouseNotConfigured = errors.New("ClickHouse not configured")

// Service provides APM business logic.
type Service struct {
	repo *Repository
	ch   *ClickHouseClient
}

// NewService creates a new APM service.
func NewService(i do.Injector) (*Service, error) {
	repo := do.MustInvoke[*Repository](i)
	ch := do.MustInvoke[*ClickHouseClient](i)
	return &Service{repo: repo, ch: ch}, nil
}

// ListTraces returns trace summaries matching the given filters.
func (s *Service) ListTraces(ctx context.Context, f TraceFilters) ([]TraceSummary, int64, error) {
	if s.ch == nil {
		return nil, 0, ErrClickHouseNotConfigured
	}
	return s.repo.ListTraces(ctx, f)
}

// GetTrace returns all spans for a given trace ID.
func (s *Service) GetTrace(ctx context.Context, traceId string) ([]Span, error) {
	if s.ch == nil {
		return nil, ErrClickHouseNotConfigured
	}
	return s.repo.GetTrace(ctx, traceId)
}

// ListServices returns aggregated metrics for all services.
func (s *Service) ListServices(ctx context.Context, start, end time.Time) ([]ServiceOverview, error) {
	if s.ch == nil {
		return nil, ErrClickHouseNotConfigured
	}
	return s.repo.ListServices(ctx, start, end)
}

// GetServiceDetail returns aggregated metrics for a single service.
func (s *Service) GetServiceDetail(ctx context.Context, serviceName string, start, end time.Time) (*ServiceDetail, error) {
	if s.ch == nil {
		return nil, ErrClickHouseNotConfigured
	}
	return s.repo.GetServiceDetail(ctx, serviceName, start, end)
}

// ListOperations returns per-operation metrics for a service.
func (s *Service) ListOperations(ctx context.Context, serviceName string, start, end time.Time) ([]OperationStats, error) {
	if s.ch == nil {
		return nil, ErrClickHouseNotConfigured
	}
	return s.repo.ListOperations(ctx, serviceName, start, end)
}

// GetTimeseries returns time-bucketed aggregation.
func (s *Service) GetTimeseries(ctx context.Context, p TimeseriesParams) ([]TimeseriesPoint, error) {
	if s.ch == nil {
		return nil, ErrClickHouseNotConfigured
	}
	return s.repo.GetTimeseries(ctx, p)
}

// GetServiceSparklines returns mini timeseries for all services.
func (s *Service) GetServiceSparklines(ctx context.Context, start, end time.Time, interval int) (map[string][]SparklinePoint, error) {
	if s.ch == nil {
		return nil, ErrClickHouseNotConfigured
	}
	return s.repo.GetServiceSparklines(ctx, start, end, interval)
}

// GetTopology returns the service call topology graph.
func (s *Service) GetTopology(ctx context.Context, start, end time.Time) (*TopologyGraph, error) {
	if s.ch == nil {
		return nil, ErrClickHouseNotConfigured
	}
	return s.repo.GetTopology(ctx, start, end)
}

// GetLogsByTraceId returns log records correlated with a trace ID.
func (s *Service) GetLogsByTraceId(ctx context.Context, traceId string) ([]TraceLog, bool, error) {
	if s.ch == nil {
		return nil, false, ErrClickHouseNotConfigured
	}
	return s.repo.GetLogsByTraceId(ctx, traceId)
}

// SearchSpans searches spans by attribute key/value.
func (s *Service) SearchSpans(ctx context.Context, p SpanSearchParams) ([]Span, int64, error) {
	if s.ch == nil {
		return nil, 0, ErrClickHouseNotConfigured
	}
	return s.repo.SearchSpans(ctx, p)
}

// GetAnalytics returns aggregated metrics grouped by the specified dimension.
func (s *Service) GetAnalytics(ctx context.Context, p AnalyticsParams) ([]AnalyticsGroup, error) {
	if s.ch == nil {
		return nil, ErrClickHouseNotConfigured
	}
	return s.repo.GetAnalytics(ctx, p)
}

// GetLatencyDistribution returns a histogram of latency distribution.
func (s *Service) GetLatencyDistribution(ctx context.Context, p LatencyDistParams) ([]LatencyBucket, error) {
	if s.ch == nil {
		return nil, ErrClickHouseNotConfigured
	}
	return s.repo.GetLatencyDistribution(ctx, p)
}

// GetErrors returns error groups aggregated by error type and message.
func (s *Service) GetErrors(ctx context.Context, start, end time.Time, service string) ([]ErrorGroup, error) {
	if s.ch == nil {
		return nil, ErrClickHouseNotConfigured
	}
	return s.repo.GetErrors(ctx, start, end, service)
}
