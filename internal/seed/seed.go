package seed

import (
	"fmt"
	"log/slog"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"metis/internal/model"
)

type Result struct {
	RolesCreated  int
	RolesSkipped  int
	MenusCreated  int
	MenusSkipped  int
	PoliciesAdded int
}

// Install performs full initialization during first-time installation.
// Seeds roles, menus, policies, default configs, and auth providers.
func Install(db *gorm.DB, enforcer *casbin.Enforcer) (*Result, error) {
	r := &Result{}

	// 1. Seed roles
	roleMap, err := seedRoles(db, r)
	if err != nil {
		return nil, err
	}

	// 2. Seed menus
	if err := seedMenuTree(db, BuiltinMenus, nil, r); err != nil {
		return nil, err
	}

	// 3. Seed Casbin policies
	if err := seedPolicies(db, enforcer, roleMap, r); err != nil {
		return nil, err
	}

	// 4. Seed ALL default configs (server_port, otel.*, system.app_name, etc.)
	if err := seedDefaultConfigs(db); err != nil {
		return nil, err
	}

	// 5. Seed default auth providers
	if err := seedAuthProviders(db); err != nil {
		return nil, err
	}

	return r, nil
}

// Sync performs incremental synchronization on subsequent startups.
// Only adds new roles/menus/policies that don't already exist.
// Does NOT overwrite existing SystemConfig values or auth providers.
func Sync(db *gorm.DB, enforcer *casbin.Enforcer) (*Result, error) {
	r := &Result{}

	// 1. Sync roles (add missing only)
	if _, err := seedRoles(db, r); err != nil {
		return nil, err
	}

	// 2. Sync menus (add missing only)
	if err := seedMenuTree(db, BuiltinMenus, nil, r); err != nil {
		return nil, err
	}

	// 3. Sync Casbin policies (re-apply all — idempotent)
	roleMap := make(map[string]*model.Role)
	for _, seed := range BuiltinRoles {
		var role model.Role
		if err := db.Where("code = ?", seed.Code).First(&role).Error; err == nil {
			roleMap[role.Code] = &role
		} else if err != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("load role %s for policy sync: %w", seed.Code, err)
		}
	}
	if err := seedPolicies(db, enforcer, roleMap, r); err != nil {
		return nil, err
	}

	return r, nil
}

func seedRoles(db *gorm.DB, r *Result) (map[string]*model.Role, error) {
	roleMap := make(map[string]*model.Role)
	for _, seed := range BuiltinRoles {
		var existing model.Role
		if err := db.Where("code = ?", seed.Code).First(&existing).Error; err == nil {
			r.RolesSkipped++
			roleMap[existing.Code] = &existing
			continue
		} else if err != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("lookup role %s: %w", seed.Code, err)
		}
		role := seed
		if err := db.Create(&role).Error; err != nil {
			return nil, fmt.Errorf("create role %s: %w", seed.Code, err)
		}
		r.RolesCreated++
		roleMap[role.Code] = &role
		slog.Info("seed: created role", "code", role.Code)
	}
	return roleMap, nil
}

func seedMenuTree(db *gorm.DB, menus []MenuSeed, parentID *uint, r *Result) error {
	for _, seed := range menus {
		var existing model.Menu
		if seed.Permission != "" {
			if err := db.Where("permission = ?", seed.Permission).First(&existing).Error; err == nil {
				r.MenusSkipped++
				if err := seedMenuTree(db, seed.Children, &existing.ID, r); err != nil {
					return err
				}
				continue
			} else if err != gorm.ErrRecordNotFound {
				return fmt.Errorf("lookup menu %s: %w", seed.Name, err)
			}
		}

		menu := seed.Menu
		menu.ParentID = parentID
		if err := db.Create(&menu).Error; err != nil {
			return fmt.Errorf("create menu %s: %w", seed.Name, err)
		}
		r.MenusCreated++
		slog.Info("seed: created menu", "name", menu.Name, "permission", menu.Permission)

		if err := seedMenuTree(db, seed.Children, &menu.ID, r); err != nil {
			return err
		}
	}
	return nil
}

func seedPolicies(db *gorm.DB, enforcer *casbin.Enforcer, roleMap map[string]*model.Role, r *Result) error {
	var allMenus []model.Menu
	if err := db.Find(&allMenus).Error; err != nil {
		return fmt.Errorf("load menus for policies: %w", err)
	}

	// Admin: all menu permissions + all API policies
	var adminPolicies [][]string
	for _, m := range allMenus {
		if m.Permission != "" {
			adminPolicies = append(adminPolicies, []string{"admin", m.Permission, "read"})
		}
	}
	for _, p := range AdminAPIPolicies {
		adminPolicies = append(adminPolicies, []string{"admin", p[0], p[1]})
	}

	if _, err := enforcer.RemoveFilteredPolicy(0, "admin"); err != nil {
		return fmt.Errorf("clear admin policies: %w", err)
	}
	if len(adminPolicies) > 0 {
		added, err := enforcer.AddPolicies(adminPolicies)
		if err != nil {
			return fmt.Errorf("add admin policies: %w", err)
		}
		if added {
			r.PoliciesAdded += len(adminPolicies)
		}
	}

	// User: basic permissions
	var userPolicies [][]string
	for _, p := range UserAPIPolicies {
		userPolicies = append(userPolicies, []string{"user", p[0], p[1]})
	}

	if _, err := enforcer.RemoveFilteredPolicy(0, "user"); err != nil {
		return fmt.Errorf("clear user policies: %w", err)
	}
	if len(userPolicies) > 0 {
		added, err := enforcer.AddPolicies(userPolicies)
		if err != nil {
			return fmt.Errorf("add user policies: %w", err)
		}
		if added {
			r.PoliciesAdded += len(userPolicies)
		}
	}
	return nil
}

// defaultConfigs are seeded if they don't already exist.
var defaultConfigs = []model.SystemConfig{
	// Scheduler
	{Key: "scheduler.history_retention_days", Value: "30", Remark: "任务执行历史保留天数，0 表示永不清理"},
	// Security
	{Key: "security.max_concurrent_sessions", Value: "5", Remark: "每用户最大并发会话数，0 表示不限制"},
	{Key: "security.password_min_length", Value: "8", Remark: "密码最小长度"},
	{Key: "security.password_require_upper", Value: "false", Remark: "密码是否要求大写字母"},
	{Key: "security.password_require_lower", Value: "false", Remark: "密码是否要求小写字母"},
	{Key: "security.password_require_number", Value: "false", Remark: "密码是否要求数字"},
	{Key: "security.password_require_special", Value: "false", Remark: "密码是否要求特殊字符"},
	{Key: "security.password_expiry_days", Value: "0", Remark: "密码过期天数，0 表示永不过期"},
	{Key: "security.login_max_attempts", Value: "5", Remark: "最大登录失败次数，0 表示不限制"},
	{Key: "security.login_lockout_minutes", Value: "30", Remark: "登录锁定时长（分钟）"},
	{Key: "security.session_timeout_minutes", Value: "10080", Remark: "会话超时时间（分钟），默认 7 天"},
	{Key: "security.require_two_factor", Value: "false", Remark: "是否强制所有用户启用两步验证"},
	{Key: "security.registration_open", Value: "false", Remark: "是否开放用户注册"},
	{Key: "security.default_role_code", Value: "", Remark: "注册用户默认角色代码"},
	{Key: "security.captcha_provider", Value: "none", Remark: "验证码类型：none, image"},
	// Audit
	{Key: "audit.retention_days_auth", Value: "90", Remark: "登录活动日志保留天数，0 表示永不清理"},
	{Key: "audit.retention_days_operation", Value: "365", Remark: "操作记录日志保留天数，0 表示永不清理"},
	{Key: "audit.retention_days_application", Value: "30", Remark: "系统事件日志保留天数，0 表示永不清理"},
	// Server
	{Key: "server_port", Value: "8080", Remark: "HTTP 服务监听端口（修改后需重启）"},
	// OpenTelemetry
	{Key: "otel.enabled", Value: "false", Remark: "是否启用 OpenTelemetry 追踪"},
	{Key: "otel.exporter_endpoint", Value: "http://localhost:4318", Remark: "OTLP HTTP 导出端点"},
	{Key: "otel.service_name", Value: "metis", Remark: "OTel 服务名称"},
	{Key: "otel.sample_rate", Value: "1.0", Remark: "Trace 采样率 (0-1)"},
	// Site
	{Key: "system.app_name", Value: "Metis", Remark: "站点名称"},
	{Key: "system.locale", Value: "zh-CN", Remark: "系统默认语言"},
	{Key: "system.timezone", Value: "UTC", Remark: "系统默认时区"},
}

func seedDefaultConfigs(db *gorm.DB) error {
	for _, cfg := range defaultConfigs {
		var existing model.SystemConfig
		if err := db.Where("\"key\" = ?", cfg.Key).First(&existing).Error; err == nil {
			continue
		} else if err != gorm.ErrRecordNotFound {
			return fmt.Errorf("lookup config %s: %w", cfg.Key, err)
		}
		if err := db.Create(&cfg).Error; err != nil {
			return fmt.Errorf("create config %s: %w", cfg.Key, err)
		}
		slog.Info("seed: created config", "key", cfg.Key)
	}
	return nil
}

var defaultAuthProviders = []model.AuthProvider{
	{ProviderKey: "github", DisplayName: "GitHub", SortOrder: 1},
	{ProviderKey: "google", DisplayName: "Google", SortOrder: 2},
}

func seedAuthProviders(db *gorm.DB) error {
	for _, p := range defaultAuthProviders {
		var existing model.AuthProvider
		if err := db.Where("provider_key = ?", p.ProviderKey).First(&existing).Error; err == nil {
			continue
		} else if err != gorm.ErrRecordNotFound {
			return fmt.Errorf("lookup auth provider %s: %w", p.ProviderKey, err)
		}
		if err := db.Create(&p).Error; err != nil {
			return fmt.Errorf("create auth provider %s: %w", p.ProviderKey, err)
		}
		slog.Info("seed: created auth provider", "key", p.ProviderKey)
	}
	return nil
}

// SetSiteName updates the system.app_name config during installation.
func SetSiteName(db *gorm.DB, name string) error {
	return db.Where("\"key\" = ?", "system.app_name").Assign(model.SystemConfig{Value: name}).FirstOrCreate(&model.SystemConfig{Key: "system.app_name"}).Error
}

// SetLocaleTimezone updates the system.locale and system.timezone configs during installation.
func SetLocaleTimezone(db *gorm.DB, locale, timezone string) error {
	if locale != "" {
		if err := db.Where("\"key\" = ?", "system.locale").Assign(model.SystemConfig{Value: locale}).FirstOrCreate(&model.SystemConfig{Key: "system.locale"}).Error; err != nil {
			return err
		}
	}
	if timezone != "" {
		if err := db.Where("\"key\" = ?", "system.timezone").Assign(model.SystemConfig{Value: timezone}).FirstOrCreate(&model.SystemConfig{Key: "system.timezone"}).Error; err != nil {
			return err
		}
	}
	return nil
}

// SetInstalled marks the system as installed.
func SetInstalled(db *gorm.DB) error {
	cfg := model.SystemConfig{Key: "app.installed", Value: "true", Remark: "系统安装标记"}
	return db.Where("\"key\" = ?", "app.installed").Assign(cfg).FirstOrCreate(&cfg).Error
}

// SetOTelConfig updates OTel-related configs during installation.
func SetOTelConfig(db *gorm.DB, enabled, endpoint, serviceName, sampleRate string) error {
	configs := []model.SystemConfig{
		{Key: "otel.enabled", Value: enabled},
		{Key: "otel.exporter_endpoint", Value: endpoint},
		{Key: "otel.service_name", Value: serviceName},
		{Key: "otel.sample_rate", Value: sampleRate},
	}
	for _, cfg := range configs {
		if err := db.Where("\"key\" = ?", cfg.Key).Assign(cfg).FirstOrCreate(&cfg).Error; err != nil {
			return err
		}
	}
	return nil
}

// IsInstalled checks if the system has been installed.
func IsInstalled(db *gorm.DB) bool {
	var cfg model.SystemConfig
	if err := db.Where("\"key\" = ? AND value = ?", "app.installed", "true").First(&cfg).Error; err != nil {
		return false
	}
	return true
}
