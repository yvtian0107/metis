package apm

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/scheduler"
)

func init() {
	app.Register(&APMApp{})
}

type APMApp struct {
	injector do.Injector
}

func (a *APMApp) Name() string { return "apm" }

func (a *APMApp) Models() []any { return nil }

func (a *APMApp) Seed(db *gorm.DB, enforcer *casbin.Enforcer, _ bool) error {
	return seedAPM(db, enforcer)
}

func (a *APMApp) Providers(i do.Injector) {
	a.injector = i
	do.Provide(i, NewClickHouseClient)
	do.Provide(i, NewRepository)
	do.Provide(i, NewService)
	do.Provide(i, NewHandler)
}

func (a *APMApp) Routes(api *gin.RouterGroup) {
	h := do.MustInvoke[*Handler](a.injector)

	traces := api.Group("/apm/traces")
	{
		traces.GET("", h.ListTraces)
		traces.GET("/:traceId", h.GetTrace)
		traces.GET("/:traceId/logs", h.GetTraceLogs)
	}

	services := api.Group("/apm/services")
	{
		services.GET("", h.ListServices)
		services.GET("/:name", h.GetServiceDetail)
	}

	api.GET("/apm/timeseries", h.GetTimeseries)
	api.GET("/apm/topology", h.GetTopology)
	api.GET("/apm/spans/search", h.SearchSpans)
	api.GET("/apm/analytics", h.GetAnalytics)
	api.GET("/apm/latency-distribution", h.GetLatencyDistribution)
	api.GET("/apm/errors", h.GetErrors)
}

func (a *APMApp) Tasks() []scheduler.TaskDef {
	return nil
}
