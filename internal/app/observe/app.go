package observe

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"
	"metis/internal/app/observe/auth"
	"metis/internal/app/observe/bootstrap"
	"metis/internal/app/observe/domain"
	"metis/internal/app/observe/token"

	"metis/internal/app"
	"metis/internal/scheduler"
)

func init() {
	app.Register(&ObserveApp{})
}

type ObserveApp struct {
	injector do.Injector
}

func (a *ObserveApp) Name() string { return "observe" }

func (a *ObserveApp) Models() []any {
	return []any{&domain.IntegrationToken{}}
}

func (a *ObserveApp) Seed(db *gorm.DB, enforcer *casbin.Enforcer, _ bool) error {
	return bootstrap.SeedObserve(db, enforcer)
}

func (a *ObserveApp) Providers(i do.Injector) {
	a.injector = i
	do.Provide(i, token.NewIntegrationTokenRepo)
	do.Provide(i, token.NewIntegrationTokenService)
	do.Provide(i, token.NewIntegrationTokenHandler)
	do.Provide(i, auth.NewAuthHandler)
}

func (a *ObserveApp) Routes(api *gin.RouterGroup) {
	tokenH := do.MustInvoke[*token.IntegrationTokenHandler](a.injector)

	// JWT + Casbin protected routes
	tokens := api.Group("/observe/tokens")
	{
		tokens.POST("", tokenH.Create)
		tokens.GET("", tokenH.List)
		tokens.DELETE("/:id", tokenH.Revoke)
	}
	api.GET("/observe/settings", tokenH.GetSettings)

	// ForwardAuth verify endpoint — bypasses JWT+Casbin, registered on raw Engine
	authH := do.MustInvoke[*auth.AuthHandler](a.injector)
	r := do.MustInvoke[*gin.Engine](a.injector)
	r.GET("/api/v1/observe/auth/verify", authH.Verify)
}

func (a *ObserveApp) Tasks() []scheduler.TaskDef {
	return nil
}
