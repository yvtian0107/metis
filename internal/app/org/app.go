package org

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/app/org/assignment"
	"metis/internal/app/org/bootstrap"
	"metis/internal/app/org/department"
	"metis/internal/app/org/domain"
	"metis/internal/app/org/position"
	"metis/internal/app/org/resolver"
	"metis/internal/app/org/tools"
	"metis/internal/database"
	"metis/internal/scheduler"
)

func init() {
	app.Register(&OrgApp{})
}

type OrgApp struct {
	injector do.Injector
}

func (a *OrgApp) Name() string { return "org" }

// GetToolRegistry implements app.ToolRegistryProvider.
func (a *OrgApp) GetToolRegistry() any {
	resolver := do.MustInvoke[app.OrgResolver](a.injector)
	return tools.NewOrgToolRegistry(resolver)
}

// Ensure OrgApp implements app.ToolRegistryProvider at compile time.
var _ app.ToolRegistryProvider = (*OrgApp)(nil)

func (a *OrgApp) Models() []any {
	return []any{&domain.Department{}, &domain.Position{}, &domain.UserPosition{}, &domain.DepartmentPosition{}}
}

func (a *OrgApp) Seed(db *gorm.DB, enforcer *casbin.Enforcer, install bool) error {
	return bootstrap.SeedOrg(db, enforcer, install)
}

func (a *OrgApp) Providers(i do.Injector) {
	a.injector = i
	// Repositories
	do.Provide(i, department.NewDepartmentRepo)
	do.Provide(i, position.NewPositionRepo)
	do.Provide(i, assignment.NewAssignmentRepo)
	// Services
	do.Provide(i, department.NewDepartmentService)
	do.Provide(i, position.NewPositionService)
	do.Provide(i, assignment.NewAssignmentService)
	// Handlers
	do.Provide(i, department.NewDepartmentHandler)
	do.Provide(i, position.NewPositionHandler)
	do.Provide(i, assignment.NewAssignmentHandler)
	// OrgResolver — unified interface for DataScope, ITSM, and AI tools
	do.ProvideValue[app.OrgResolver](i, resolver.NewOrgResolver(
		do.MustInvoke[*assignment.AssignmentService](i),
		do.MustInvoke[*assignment.AssignmentRepo](i),
		do.MustInvoke[*database.DB](i).DB,
	))
}

func (a *OrgApp) Routes(api *gin.RouterGroup) {
	deptH := do.MustInvoke[*department.DepartmentHandler](a.injector)
	posH := do.MustInvoke[*position.PositionHandler](a.injector)
	assignH := do.MustInvoke[*assignment.AssignmentHandler](a.injector)

	org := api.Group("/org")
	{
		// Departments
		org.POST("/departments", deptH.Create)
		org.GET("/departments", deptH.List)
		org.GET("/departments/tree", deptH.Tree)
		org.GET("/departments/:id", deptH.Get)
		org.PUT("/departments/:id", deptH.Update)
		org.DELETE("/departments/:id", deptH.Delete)
		org.GET("/departments/:id/positions", deptH.GetAllowedPositions)
		org.PUT("/departments/:id/positions", deptH.SetAllowedPositions)

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
		org.PUT("/users/:id/departments/:deptId/positions", assignH.SetUserDeptPositions)
		org.GET("/users", assignH.ListUsers)
	}
}

func (a *OrgApp) Tasks() []scheduler.TaskDef {
	return nil
}
