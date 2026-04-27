package license

import (
	"context"
	"encoding/json"
	"metis/internal/app/license/bootstrap"
	"metis/internal/app/license/certificate"
	"metis/internal/app/license/domain"
	"metis/internal/app/license/licensee"
	"metis/internal/app/license/product"
	"metis/internal/app/license/registration"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/scheduler"
)

func init() {
	app.Register(&LicenseApp{})
}

type LicenseApp struct {
	injector do.Injector
}

func (a *LicenseApp) Name() string { return "license" }

func (a *LicenseApp) Models() []any {
	return []any{&domain.Product{}, &domain.Plan{}, &domain.ProductKey{}, &domain.Licensee{}, &domain.License{}, &domain.LicenseRegistration{}}
}

func (a *LicenseApp) Seed(db *gorm.DB, enforcer *casbin.Enforcer, _ bool) error {
	return bootstrap.SeedLicense(db, enforcer)
}

func (a *LicenseApp) Providers(i do.Injector) {
	a.injector = i
	do.Provide(i, product.NewProductRepo)
	do.Provide(i, product.NewPlanRepo)
	do.Provide(i, product.NewProductKeyRepo)
	do.Provide(i, licensee.NewLicenseeRepo)
	do.Provide(i, certificate.NewLicenseRepo)
	do.Provide(i, registration.NewLicenseRegistrationRepo)
	do.Provide(i, product.NewProductService)
	do.Provide(i, product.NewPlanService)
	do.Provide(i, licensee.NewLicenseeService)
	do.Provide(i, certificate.NewLicenseService)
	do.Provide(i, func(i do.Injector) (product.LicenseOperations, error) {
		return do.MustInvoke[*certificate.LicenseService](i), nil
	})
	do.Provide(i, product.NewProductHandler)
	do.Provide(i, product.NewPlanHandler)
	do.Provide(i, licensee.NewLicenseeHandler)
	do.Provide(i, certificate.NewLicenseHandler)
}

func (a *LicenseApp) Routes(api *gin.RouterGroup) {
	productH := do.MustInvoke[*product.ProductHandler](a.injector)
	planH := do.MustInvoke[*product.PlanHandler](a.injector)
	licenseeH := do.MustInvoke[*licensee.LicenseeHandler](a.injector)
	licenseH := do.MustInvoke[*certificate.LicenseHandler](a.injector)

	products := api.Group("/license/products")
	{
		products.POST("", productH.Create)
		products.GET("", productH.List)
		products.GET("/:id", productH.Get)
		products.PUT("/:id", productH.Update)
		products.PUT("/:id/schema", productH.UpdateSchema)
		products.PATCH("/:id/status", productH.UpdateStatus)
		products.POST("/:id/rotate-key", productH.RotateKey)
		products.GET("/:id/rotate-key-impact", productH.RotateKeyImpact)
		products.POST("/:id/bulk-reissue", productH.BulkReissue)
		products.GET("/:id/public-key", productH.GetPublicKey)
		products.POST("/:id/plans", planH.Create)
	}

	plans := api.Group("/license/plans")
	{
		plans.PUT("/:id", planH.Update)
		plans.DELETE("/:id", planH.Delete)
		plans.PATCH("/:id/default", planH.SetDefault)
	}

	licensees := api.Group("/license/licensees")
	{
		licensees.POST("", licenseeH.Create)
		licensees.GET("", licenseeH.List)
		licensees.GET("/:id", licenseeH.Get)
		licensees.PUT("/:id", licenseeH.Update)
		licensees.PATCH("/:id/status", licenseeH.UpdateStatus)
	}

	licenses := api.Group("/license/licenses")
	{
		licenses.POST("", licenseH.Issue)
		licenses.GET("", licenseH.List)
		licenses.GET("/:id", licenseH.Get)
		licenses.POST("/:id/renew", licenseH.Renew)
		licenses.POST("/:id/upgrade", licenseH.Upgrade)
		licenses.POST("/:id/suspend", licenseH.Suspend)
		licenses.POST("/:id/reactivate", licenseH.Reactivate)
		licenses.PATCH("/:id/revoke", licenseH.Revoke)
		licenses.GET("/:id/export", licenseH.Export)
	}

	registrations := api.Group("/license/registrations")
	{
		registrations.POST("", licenseH.CreateRegistration)
		registrations.GET("", licenseH.ListRegistrations)
		registrations.POST("/generate", licenseH.GenerateRegistration)
	}
}

func (a *LicenseApp) Tasks() []scheduler.TaskDef {
	licenseSvc := do.MustInvoke[*certificate.LicenseService](a.injector)
	return []scheduler.TaskDef{
		{
			Name:        "license-expired-check",
			Type:        scheduler.TypeScheduled,
			Description: "Check and mark expired licenses daily",
			CronExpr:    "0 2 * * *",
			Handler: func(ctx context.Context, payload json.RawMessage) error {
				return licenseSvc.CheckExpiredLicenses()
			},
		},
		{
			Name:        "license-registration-cleanup",
			Type:        scheduler.TypeScheduled,
			Description: "Cleanup expired and unbound license registrations daily",
			CronExpr:    "0 3 * * *",
			Handler: func(ctx context.Context, payload json.RawMessage) error {
				return licenseSvc.CleanupExpiredRegistrations()
			},
		},
	}
}
