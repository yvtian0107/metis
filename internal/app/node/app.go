package node

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/app/node/bootstrap"
	"metis/internal/app/node/command"
	"metis/internal/app/node/domain"
	nodelog "metis/internal/app/node/log"
	nodenode "metis/internal/app/node/node"
	nodeprocess "metis/internal/app/node/process"
	"metis/internal/app/node/processdef"
	"metis/internal/app/node/sidecar"
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
	return []any{&domain.Node{}, &domain.ProcessDef{}, &domain.NodeProcess{}, &domain.NodeCommand{}, &domain.NodeProcessLog{}}
}

func (a *NodeApp) Seed(db *gorm.DB, enforcer *casbin.Enforcer, _ bool) error {
	return bootstrap.SeedNode(db, enforcer)
}

func (a *NodeApp) Providers(i do.Injector) {
	a.injector = i
	do.Provide(i, nodenode.NewNodeRepo)
	do.Provide(i, processdef.NewProcessDefRepo)
	do.Provide(i, nodeprocess.NewNodeProcessRepo)
	do.Provide(i, command.NewNodeCommandRepo)
	do.Provide(i, nodelog.NewNodeProcessLogRepo)
	do.Provide(i, func(i do.Injector) (*nodenode.NodeHub, error) {
		nodeRepo := do.MustInvoke[*nodenode.NodeRepo](i)
		return nodenode.NewNodeHub(nodeRepo), nil
	})
	do.ProvideValue[nodeprocess.NodeReader](i, do.MustInvoke[*nodenode.NodeRepo](i))
	do.ProvideValue[nodeprocess.ProcessDefReader](i, do.MustInvoke[*processdef.ProcessDefRepo](i))
	do.ProvideValue[nodeprocess.CommandCreator](i, do.MustInvoke[*command.NodeCommandRepo](i))
	do.ProvideValue[nodeprocess.CommandPusher](i, do.MustInvoke[*nodenode.NodeHub](i))
	do.ProvideValue[processdef.NodeProcessLister](i, do.MustInvoke[*nodeprocess.NodeProcessRepo](i))
	do.ProvideValue[processdef.CommandCreator](i, do.MustInvoke[*command.NodeCommandRepo](i))
	do.ProvideValue[processdef.CommandPusher](i, do.MustInvoke[*nodenode.NodeHub](i))
	do.ProvideValue[nodelog.ProcessDefFinder](i, do.MustInvoke[*processdef.ProcessDefRepo](i))
	do.Provide(i, nodenode.NewNodeService)
	do.Provide(i, processdef.NewProcessDefService)
	do.Provide(i, nodeprocess.NewNodeProcessService)
	do.Provide(i, sidecar.NewSidecarService)
	do.Provide(i, nodelog.NewNodeProcessLogService)
	do.Provide(i, nodenode.NewNodeHandler)
	do.Provide(i, processdef.NewProcessDefHandler)
	do.Provide(i, nodeprocess.NewNodeProcessHandler)
	do.Provide(i, sidecar.NewSidecarHandler)
}

func (a *NodeApp) Routes(api *gin.RouterGroup) {
	nodeH := do.MustInvoke[*nodenode.NodeHandler](a.injector)
	processDefH := do.MustInvoke[*processdef.ProcessDefHandler](a.injector)
	nodeProcessH := do.MustInvoke[*nodeprocess.NodeProcessHandler](a.injector)
	sidecarH := do.MustInvoke[*sidecar.SidecarHandler](a.injector)

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

	// Sidecar routes (domain.Node Token auth, bypass JWT+Casbin)
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
	sidecarSvc := do.MustInvoke[*sidecar.SidecarService](a.injector)
	logSvc := do.MustInvoke[*nodelog.NodeProcessLogService](a.injector)
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
