package trace

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

// Handler handles APM HTTP requests.
type Handler struct {
	svc *Service
}

// NewHandler creates a new APM handler.
func NewHandler(i do.Injector) (*Handler, error) {
	svc := do.MustInvoke[*Service](i)
	return &Handler{svc: svc}, nil
}

// ListTraces handles GET /api/v1/apm/traces
func (h *Handler) ListTraces(c *gin.Context) {
	startStr := c.Query("start")
	endStr := c.Query("end")
	if startStr == "" || endStr == "" {
		handler.Fail(c, http.StatusBadRequest, "start and end are required")
		return
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid start time format")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid end time format")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	f := TraceFilters{
		Start:     start,
		End:       end,
		Service:   c.Query("service"),
		Operation: c.Query("operation"),
		Status:    c.Query("status"),
		Page:      page,
		PageSize:  pageSize,
	}

	if v := c.Query("duration_min"); v != "" {
		d, err := strconv.ParseFloat(v, 64)
		if err == nil {
			f.DurationMin = &d
		}
	}
	if v := c.Query("duration_max"); v != "" {
		d, err := strconv.ParseFloat(v, 64)
		if err == nil {
			f.DurationMax = &d
		}
	}

	traces, total, err := h.svc.ListTraces(c.Request.Context(), f)
	if err != nil {
		if errors.Is(err, ErrClickHouseNotConfigured) {
			handler.Fail(c, http.StatusServiceUnavailable, "ClickHouse not configured")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, gin.H{
		"items": traces,
		"total": total,
		"page":  page,
	})
}

// GetTrace handles GET /api/v1/apm/traces/:traceId
func (h *Handler) GetTrace(c *gin.Context) {
	traceId := c.Param("traceId")
	if traceId == "" {
		handler.Fail(c, http.StatusBadRequest, "traceId is required")
		return
	}

	spans, err := h.svc.GetTrace(c.Request.Context(), traceId)
	if err != nil {
		if errors.Is(err, ErrClickHouseNotConfigured) {
			handler.Fail(c, http.StatusServiceUnavailable, "ClickHouse not configured")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, gin.H{
		"spans": spans,
	})
}

// ListServices handles GET /api/v1/apm/services
func (h *Handler) ListServices(c *gin.Context) {
	startStr := c.Query("start")
	endStr := c.Query("end")
	if startStr == "" || endStr == "" {
		handler.Fail(c, http.StatusBadRequest, "start and end are required")
		return
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid start time format")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid end time format")
		return
	}

	services, err := h.svc.ListServices(c.Request.Context(), start, end)
	if err != nil {
		if errors.Is(err, ErrClickHouseNotConfigured) {
			handler.Fail(c, http.StatusServiceUnavailable, "ClickHouse not configured")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Also fetch sparklines (use 1h buckets for 7d default, auto-detect from range)
	rangeSec := int(end.Sub(start).Seconds())
	interval := pickInterval(rangeSec)
	sparklines, err := h.svc.GetServiceSparklines(c.Request.Context(), start, end, interval)
	if err != nil {
		sparklines = nil // non-fatal
	}

	handler.OK(c, gin.H{
		"services":   services,
		"sparklines": sparklines,
	})
}

// GetServiceDetail handles GET /api/v1/apm/services/:name
func (h *Handler) GetServiceDetail(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		handler.Fail(c, http.StatusBadRequest, "service name is required")
		return
	}

	startStr := c.Query("start")
	endStr := c.Query("end")
	if startStr == "" || endStr == "" {
		handler.Fail(c, http.StatusBadRequest, "start and end are required")
		return
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid start time format")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid end time format")
		return
	}

	detail, err := h.svc.GetServiceDetail(c.Request.Context(), name, start, end)
	if err != nil {
		if errors.Is(err, ErrClickHouseNotConfigured) {
			handler.Fail(c, http.StatusServiceUnavailable, "ClickHouse not configured")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, detail)
}

// GetTimeseries handles GET /api/v1/apm/timeseries
func (h *Handler) GetTimeseries(c *gin.Context) {
	startStr := c.Query("start")
	endStr := c.Query("end")
	if startStr == "" || endStr == "" {
		handler.Fail(c, http.StatusBadRequest, "start and end are required")
		return
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid start time format")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid end time format")
		return
	}

	rangeSec := int(end.Sub(start).Seconds())
	interval := pickInterval(rangeSec)
	if v := c.Query("interval"); v != "" {
		if parsed, e := strconv.Atoi(v); e == nil && parsed > 0 {
			interval = parsed
		}
	}

	p := TimeseriesParams{
		Start:     start,
		End:       end,
		Service:   c.Query("service"),
		Operation: c.Query("operation"),
		Interval:  interval,
	}

	points, err := h.svc.GetTimeseries(c.Request.Context(), p)
	if err != nil {
		if errors.Is(err, ErrClickHouseNotConfigured) {
			handler.Fail(c, http.StatusServiceUnavailable, "ClickHouse not configured")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, gin.H{
		"points":   points,
		"interval": interval,
	})
}

// pickInterval selects a time bucket interval (seconds) based on total range.
func pickInterval(rangeSec int) int {
	switch {
	case rangeSec <= 900: // 15m
		return 15
	case rangeSec <= 3600: // 1h
		return 60
	case rangeSec <= 21600: // 6h
		return 300
	case rangeSec <= 86400: // 24h
		return 900
	default: // 7d+
		return 3600
	}
}

// GetTopology handles GET /api/v1/apm/topology
func (h *Handler) GetTopology(c *gin.Context) {
	startStr := c.Query("start")
	endStr := c.Query("end")
	if startStr == "" || endStr == "" {
		handler.Fail(c, http.StatusBadRequest, "start and end are required")
		return
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid start time format")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid end time format")
		return
	}

	graph, err := h.svc.GetTopology(c.Request.Context(), start, end)
	if err != nil {
		if errors.Is(err, ErrClickHouseNotConfigured) {
			handler.Fail(c, http.StatusServiceUnavailable, "ClickHouse not configured")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, graph)
}

// GetTraceLogs handles GET /api/v1/apm/traces/:traceId/logs
func (h *Handler) GetTraceLogs(c *gin.Context) {
	traceId := c.Param("traceId")
	if traceId == "" {
		handler.Fail(c, http.StatusBadRequest, "traceId is required")
		return
	}

	logs, available, err := h.svc.GetLogsByTraceId(c.Request.Context(), traceId)
	if err != nil {
		if errors.Is(err, ErrClickHouseNotConfigured) {
			handler.Fail(c, http.StatusServiceUnavailable, "ClickHouse not configured")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, gin.H{
		"logs":          logs,
		"logsAvailable": available,
	})
}

// SearchSpans handles GET /api/v1/apm/spans/search
func (h *Handler) SearchSpans(c *gin.Context) {
	startStr := c.Query("start")
	endStr := c.Query("end")
	if startStr == "" || endStr == "" {
		handler.Fail(c, http.StatusBadRequest, "start and end are required")
		return
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid start time format")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid end time format")
		return
	}

	key := c.Query("key")
	if key == "" {
		handler.Fail(c, http.StatusBadRequest, "key is required")
		return
	}

	op := c.DefaultQuery("op", "eq")
	if op != "eq" && op != "contains" && op != "exists" {
		handler.Fail(c, http.StatusBadRequest, "op must be eq, contains, or exists")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	spans, total, err := h.svc.SearchSpans(c.Request.Context(), SpanSearchParams{
		Start: start, End: end, Key: key, Value: c.Query("value"), Op: op,
		Page: page, Size: pageSize,
	})
	if err != nil {
		if errors.Is(err, ErrClickHouseNotConfigured) {
			handler.Fail(c, http.StatusServiceUnavailable, "ClickHouse not configured")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, gin.H{"items": spans, "total": total, "page": page})
}

// GetAnalytics handles GET /api/v1/apm/analytics
func (h *Handler) GetAnalytics(c *gin.Context) {
	startStr := c.Query("start")
	endStr := c.Query("end")
	if startStr == "" || endStr == "" {
		handler.Fail(c, http.StatusBadRequest, "start and end are required")
		return
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid start time format")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid end time format")
		return
	}

	groupBy := c.DefaultQuery("groupBy", "service")

	groups, err := h.svc.GetAnalytics(c.Request.Context(), AnalyticsParams{
		Start: start, End: end, GroupBy: groupBy,
		Service: c.Query("service"), Operation: c.Query("operation"),
	})
	if err != nil {
		if errors.Is(err, ErrClickHouseNotConfigured) {
			handler.Fail(c, http.StatusServiceUnavailable, "ClickHouse not configured")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, gin.H{"groups": groups})
}

// GetLatencyDistribution handles GET /api/v1/apm/latency-distribution
func (h *Handler) GetLatencyDistribution(c *gin.Context) {
	startStr := c.Query("start")
	endStr := c.Query("end")
	if startStr == "" || endStr == "" {
		handler.Fail(c, http.StatusBadRequest, "start and end are required")
		return
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid start time format")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid end time format")
		return
	}

	buckets, _ := strconv.Atoi(c.DefaultQuery("buckets", "20"))
	if buckets < 1 || buckets > 100 {
		buckets = 20
	}

	data, err := h.svc.GetLatencyDistribution(c.Request.Context(), LatencyDistParams{
		Start: start, End: end,
		Service: c.Query("service"), Operation: c.Query("operation"),
		Buckets: buckets,
	})
	if err != nil {
		if errors.Is(err, ErrClickHouseNotConfigured) {
			handler.Fail(c, http.StatusServiceUnavailable, "ClickHouse not configured")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, gin.H{"buckets": data})
}

// GetErrors handles GET /api/v1/apm/errors
func (h *Handler) GetErrors(c *gin.Context) {
	startStr := c.Query("start")
	endStr := c.Query("end")
	if startStr == "" || endStr == "" {
		handler.Fail(c, http.StatusBadRequest, "start and end are required")
		return
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid start time format")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid end time format")
		return
	}

	groups, err := h.svc.GetErrors(c.Request.Context(), start, end, c.Query("service"))
	if err != nil {
		if errors.Is(err, ErrClickHouseNotConfigured) {
			handler.Fail(c, http.StatusServiceUnavailable, "ClickHouse not configured")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, gin.H{"errors": groups})
}
