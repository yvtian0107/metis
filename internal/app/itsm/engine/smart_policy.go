package engine

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type SmartDecisionPolicy interface {
	Apply(ctx context.Context, e *SmartEngine, tx *gorm.DB, ticketID uint, plan *DecisionPlan, svc *serviceModel) (bool, error)
}

type SmartDecisionPolicyFunc func(ctx context.Context, e *SmartEngine, tx *gorm.DB, ticketID uint, plan *DecisionPlan, svc *serviceModel) (bool, error)

func (f SmartDecisionPolicyFunc) Apply(ctx context.Context, e *SmartEngine, tx *gorm.DB, ticketID uint, plan *DecisionPlan, svc *serviceModel) (bool, error) {
	return f(ctx, e, tx, ticketID, plan, svc)
}

func builtInSmartDecisionPolicies() []SmartDecisionPolicy {
	return []SmartDecisionPolicy{
		SmartDecisionPolicyFunc(dbBackupWhitelistPolicy),
		SmartDecisionPolicyFunc(bossSerialChangePolicy),
		SmartDecisionPolicyFunc(accessPurposeRoutePolicy),
	}
}

func dbBackupWhitelistPolicy(ctx context.Context, e *SmartEngine, tx *gorm.DB, ticketID uint, plan *DecisionPlan, svc *serviceModel) (bool, error) {
	if !looksLikeDBBackupWhitelistSpec(svc.CollaborationSpec) {
		return false, nil
	}
	return true, e.applyDBBackupWhitelistGuard(ctx, tx, ticketID, plan, svc)
}

func bossSerialChangePolicy(ctx context.Context, e *SmartEngine, tx *gorm.DB, ticketID uint, plan *DecisionPlan, svc *serviceModel) (bool, error) {
	if !looksLikeBossSerialChangeSpec(svc.CollaborationSpec) {
		return false, nil
	}
	return true, e.applyBossSerialChangeGuard(tx, ticketID, plan)
}

func accessPurposeRoutePolicy(ctx context.Context, e *SmartEngine, tx *gorm.DB, ticketID uint, plan *DecisionPlan, svc *serviceModel) (bool, error) {
	expectedPosition, ok, err := collaborationSpecAccessPurposePosition(tx, ticketID, svc.CollaborationSpec)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if expectedPosition == "" {
		return true, fmt.Errorf("form.access_reason/form.operation_purpose 缺失、为空或未命中协作规范定义的访问原因分支；不得高置信结束或选择单一路由")
	}
	return true, e.applySingleHumanRouteGuard(tx, ticketID, plan, expectedPosition, "访问目的已命中协作规范岗位分支")
}
