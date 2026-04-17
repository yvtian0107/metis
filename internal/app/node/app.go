package node

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/scheduler"
)

func init() {
	app.Register(&NodeApp{})
}

type NodeApp struct {
	injector do.Injector
}

func (a *NodeApp) Name() string { return "node" }

func (a *NodeApp) Models() []any {
	return []any{&Node{}, &ProcessDef{}, &NodeProcess{}, &NodeCommand{}, &NodeProcessLog{}}
}

func (a *NodeApp) Seed(db *gorm.DB, enforcer *casbin.Enforcer, _ bool) error {
	return seedNode(db, enforcer)
}

func (a *NodeApp) Providers(i do.Injector) {
	a.injector = i
	do.Provide(i, NewNodeRepo)
	do.Provide(i, NewProcessDefRepo)
	do.Provide(i, NewNodeProcessRepo)
	do.Provide(i, NewNodeCommandRepo)
	do.Provide(i, NewNodeProcessLogRepo)
	do.Provide(i, func(i do.Injector) (*NodeHub, error) {
		nodeRepo := do.MustInvoke[*NodeRepo](i)
		return NewNodeHub(nodeRepo), nil
	})
	do.Provide(i, NewNodeService)
	do.Provide(i, NewProcessDefService)
	do.Provide(i, NewNodeProcessService)
	do.Provide(i, NewSidecarService)
	do.Provide(i, NewNodeProcessLogService)
	do.Provide(i, NewNodeHandler)
	do.Provide(i, NewProcessDefHandler)
	do.Provide(i, NewNodeProcessHandler)
	do.Provide(i, NewSidecarHandler)
}

func (a *NodeApp) Routes(api *gin.RouterGroup) {
	nodeH := do.MustInvoke[*NodeHandler](a.injector)
	processDefH := do.MustInvoke[*ProcessDefHandler](a.injector)
	nodeProcessH := do.MustInvoke[*NodeProcessHandler](a.injector)
	sidecarH := do.MustInvoke[*SidecarHandler](a.injector)

	// Admin routes (JWT + Casbin protected)
	nodes := api.Group("/nodes")
	{
		nodes.POST("", nodeH.Create)
		nodes.GET("", nodeH.List)
		nodes.GET("/:id", nodeH.Get)
		nodes.PUT("/:id", nodeH.Update)
		nodes.DELETE("/:id", nodeH.Delete)
		nodes.POST("/:id/rotate-token", nodeH.RotateToken)
		nodes.GET("/:id/commands", nodeH.ListCommands)
	}

	processDefs := api.Group("/process-defs")
	{
		processDefs.POST("", processDefH.Create)
		processDefs.GET("", processDefH.List)
		processDefs.GET("/:id", processDefH.Get)
		processDefs.PUT("/:id", processDefH.Update)
		processDefs.DELETE("/:id", processDefH.Delete)
		processDefs.GET("/:id/nodes", processDefH.ListNodes)
	}

	nodeProcesses := api.Group("/nodes/:id/processes")
	{
		nodeProcesses.POST("", nodeProcessH.Bind)
		nodeProcesses.GET("", nodeProcessH.List)
		nodeProcesses.DELETE("/:processId", nodeProcessH.Unbind)
		nodeProcesses.POST("/:processId/start", nodeProcessH.Start)
		nodeProcesses.POST("/:processId/stop", nodeProcessH.Stop)
		nodeProcesses.POST("/:processId/restart", nodeProcessH.Restart)
		nodeProcesses.POST("/:processId/reload", nodeProcessH.Reload)
		nodeProcesses.GET("/:processId/logs", nodeProcessH.Logs)
	}

	// Sidecar routes (Node Token auth, bypass JWT+Casbin)
	// Access gin.Engine from IOC to register outside authed group
	r := do.MustInvoke[*gin.Engine](a.injector)
	sidecar := r.Group("/api/v1/nodes/sidecar", sidecarH.TokenAuth())
	{
		sidecar.POST("/register", sidecarH.Register)
		sidecar.POST("/heartbeat", sidecarH.Heartbeat)
		sidecar.GET("/stream", sidecarH.Stream)
		sidecar.GET("/commands", sidecarH.PollCommands)
		sidecar.POST("/commands/:id/ack", sidecarH.AckCommand)
		sidecar.GET("/configs/:name", sidecarH.DownloadConfig)
		sidecar.POST("/logs", sidecarH.UploadLogs)
	}
}

func (a *NodeApp) Tasks() []scheduler.TaskDef {
	sidecarSvc := do.MustInvoke[*SidecarService](a.injector)
	logSvc := do.MustInvoke[*NodeProcessLogService](a.injector)
	return []scheduler.TaskDef{
		{
			Name:        "node-offline-detection",
			Type:        scheduler.TypeScheduled,
			Description: "Detect offline nodes by checking heartbeat timeout",
			CronExpr:    "* * * * *",
			Handler:     sidecarSvc.DetectOfflineNodes,
		},
		{
			Name:        "node-command-cleanup",
			Type:        scheduler.TypeScheduled,
			Description: "Clean up expired pending commands",
			CronExpr:    "*/5 * * * *",
			Handler:     sidecarSvc.CleanupExpiredCommands,
		},
		{
			Name:        "node-log-cleanup",
			Type:        scheduler.TypeScheduled,
			Description: "Clean up old node process logs",
			CronExpr:    "0 3 * * *",
			Handler:     logSvc.CleanupOldLogs,
		},
	}
}
