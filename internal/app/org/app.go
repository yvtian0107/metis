package org

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/scheduler"
)

func init() {
	app.Register(&OrgApp{})
}

type OrgApp struct {
	injector do.Injector
}

func (a *OrgApp) Name() string { return "org" }

func (a *OrgApp) Models() []any {
	return []any{&Department{}, &Position{}, &UserPosition{}}
}

func (a *OrgApp) Seed(db *gorm.DB, enforcer *casbin.Enforcer, install bool) error {
	return seedOrg(db, enforcer, install)
}

func (a *OrgApp) Providers(i do.Injector) {
	a.injector = i
	// Repositories
	do.Provide(i, NewDepartmentRepo)
	do.Provide(i, NewPositionRepo)
	do.Provide(i, NewAssignmentRepo)
	// Services
	do.Provide(i, NewDepartmentService)
	do.Provide(i, NewPositionService)
	do.Provide(i, NewAssignmentService)
	// Handlers
	do.Provide(i, NewDepartmentHandler)
	do.Provide(i, NewPositionHandler)
	do.Provide(i, NewAssignmentHandler)
	// OrgScopeResolver — satisfies app.OrgScopeResolver interface for DataScopeMiddleware
	do.ProvideValue[app.OrgScopeResolver](i, &OrgScopeResolverImpl{
		svc: do.MustInvoke[*AssignmentService](i),
	})
	// OrgUserResolver — satisfies app.OrgUserResolver for multi-dimensional participant matching
	do.ProvideValue[app.OrgUserResolver](i, &OrgUserResolverImpl{
		repo: do.MustInvoke[*AssignmentRepo](i),
	})
}

func (a *OrgApp) Routes(api *gin.RouterGroup) {
	deptH := do.MustInvoke[*DepartmentHandler](a.injector)
	posH := do.MustInvoke[*PositionHandler](a.injector)
	assignH := do.MustInvoke[*AssignmentHandler](a.injector)

	org := api.Group("/org")
	{
		// Departments
		org.POST("/departments", deptH.Create)
		org.GET("/departments", deptH.List)
		org.GET("/departments/tree", deptH.Tree)
		org.GET("/departments/:id", deptH.Get)
		org.PUT("/departments/:id", deptH.Update)
		org.DELETE("/departments/:id", deptH.Delete)

		// Positions
		org.POST("/positions", posH.Create)
		org.GET("/positions", posH.List)
		org.GET("/positions/:id", posH.Get)
		org.PUT("/positions/:id", posH.Update)
		org.DELETE("/positions/:id", posH.Delete)

		// Assignments
		org.GET("/users/:id/positions", assignH.GetUserPositions)
		org.POST("/users/:id/positions", assignH.AddUserPosition)
		org.PUT("/users/:id/positions/:assignmentId", assignH.UpdateUserPosition)
		org.DELETE("/users/:id/positions/:assignmentId", assignH.RemoveUserPosition)
		org.PUT("/users/:id/positions/:assignmentId/primary", assignH.SetPrimary)
		org.GET("/users", assignH.ListUsers)
	}
}

func (a *OrgApp) Tasks() []scheduler.TaskDef {
	return nil
}
