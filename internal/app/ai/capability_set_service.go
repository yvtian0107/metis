package ai

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/handler"
)

var (
	ErrCapabilitySetNotFound = errors.New("capability set not found")
	ErrInvalidCapabilityType = errors.New("invalid capability type")
)

var validCapabilityTypes = map[string]bool{
	CapabilityTypeTool:           true,
	CapabilityTypeMCP:            true,
	CapabilityTypeSkill:          true,
	CapabilityTypeKnowledgeBase:  true,
	CapabilityTypeKnowledgeGraph: true,
}

type CapabilitySetRepo struct {
	db *database.DB
}

func NewCapabilitySetRepo(i do.Injector) (*CapabilitySetRepo, error) {
	return &CapabilitySetRepo{db: do.MustInvoke[*database.DB](i)}, nil
}

type CapabilitySetListParams struct {
	Type string
}

func (r *CapabilitySetRepo) List(params CapabilitySetListParams) ([]CapabilitySet, error) {
	if err := ensureDefaultCapabilitySets(r.db.DB); err != nil {
		return nil, err
	}
	q := r.db.Where("is_active = ?", true)
	if params.Type != "" {
		if !validCapabilityTypes[params.Type] {
			return nil, ErrInvalidCapabilityType
		}
		q = q.Where("type = ?", params.Type)
	}
	var sets []CapabilitySet
	if err := q.Order("type ASC, sort ASC, id ASC").Find(&sets).Error; err != nil {
		return nil, err
	}
	return sets, nil
}

func (r *CapabilitySetRepo) ItemsForSet(set CapabilitySet) ([]CapabilitySetItemResponse, error) {
	return capabilitySetItemsForSet(r.db.DB, set)
}

type CapabilitySetService struct {
	repo *CapabilitySetRepo
}

func NewCapabilitySetService(i do.Injector) (*CapabilitySetService, error) {
	return &CapabilitySetService{repo: do.MustInvoke[*CapabilitySetRepo](i)}, nil
}

func (s *CapabilitySetService) List(params CapabilitySetListParams) ([]CapabilitySetResponse, error) {
	sets, err := s.repo.List(params)
	if err != nil {
		return nil, err
	}
	responses := make([]CapabilitySetResponse, 0, len(sets))
	for _, set := range sets {
		items, err := s.repo.ItemsForSet(set)
		if err != nil {
			return nil, err
		}
		responses = append(responses, set.ToResponse(items))
	}
	return responses, nil
}

type CapabilitySetHandler struct {
	svc *CapabilitySetService
}

func NewCapabilitySetHandler(i do.Injector) (*CapabilitySetHandler, error) {
	return &CapabilitySetHandler{svc: do.MustInvoke[*CapabilitySetService](i)}, nil
}

func (h *CapabilitySetHandler) List(c *gin.Context) {
	items, err := h.svc.List(CapabilitySetListParams{Type: c.Query("type")})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrInvalidCapabilityType) {
			status = http.StatusBadRequest
		}
		handler.Fail(c, status, err.Error())
		return
	}
	handler.OK(c, gin.H{"items": items, "total": len(items)})
}

func ensureDefaultCapabilitySets(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := ensureToolCapabilitySets(tx); err != nil {
			return err
		}
		if err := ensureSingleDefaultCapabilitySet(tx, CapabilityTypeMCP, "MCP 服务", "连接 MCP 服务，把远程能力与资源暴露给智能体。", 100, &MCPServer{}); err != nil {
			return err
		}
		if err := ensureSingleDefaultCapabilitySet(tx, CapabilityTypeSkill, "技能包", "复用预设技能包，为智能体补充特定任务模板与方法。", 110, &Skill{}); err != nil {
			return err
		}
		if err := ensureKnowledgeDefaultCapabilitySet(tx, CapabilityTypeKnowledgeBase, "知识库", "绑定可检索的知识库内容，为对话回答提供稳定依据。", 120, AssetCategoryKB); err != nil {
			return err
		}
		return ensureKnowledgeDefaultCapabilitySet(tx, CapabilityTypeKnowledgeGraph, "知识图谱", "绑定结构化知识图谱，用于补充关系推理与图谱检索。", 130, AssetCategoryKG)
	})
}

func ensureToolCapabilitySets(tx *gorm.DB) error {
	var tools []Tool
	if err := tx.Order("toolkit ASC, id ASC").Find(&tools).Error; err != nil {
		return err
	}
	byToolkit := map[string][]uint{}
	var toolkitOrder []string
	for _, tool := range tools {
		key := tool.Toolkit
		if key == "" {
			key = "general"
		}
		if _, ok := byToolkit[key]; !ok {
			toolkitOrder = append(toolkitOrder, key)
		}
		byToolkit[key] = append(byToolkit[key], tool.ID)
	}
	sort.Strings(toolkitOrder)
	for idx, toolkit := range toolkitOrder {
		set, err := ensureCapabilitySet(tx, CapabilityTypeTool, toolkit, "", "wrench", idx*10)
		if err != nil {
			return err
		}
		if err := ensureCapabilitySetItems(tx, set.ID, byToolkit[toolkit]); err != nil {
			return err
		}
	}
	return nil
}

func ensureSingleDefaultCapabilitySet(tx *gorm.DB, typ, name, description string, sortOrder int, model any) error {
	set, err := ensureCapabilitySet(tx, typ, name, description, "", sortOrder)
	if err != nil {
		return err
	}
	var ids []uint
	if err := tx.Model(model).Order("id ASC").Pluck("id", &ids).Error; err != nil {
		return err
	}
	return ensureCapabilitySetItems(tx, set.ID, ids)
}

func ensureKnowledgeDefaultCapabilitySet(tx *gorm.DB, typ, name, description string, sortOrder int, category string) error {
	set, err := ensureCapabilitySet(tx, typ, name, description, "", sortOrder)
	if err != nil {
		return err
	}
	var ids []uint
	if err := tx.Model(&KnowledgeAsset{}).Where("category = ?", category).Order("id ASC").Pluck("id", &ids).Error; err != nil {
		return err
	}
	return ensureCapabilitySetItems(tx, set.ID, ids)
}

func ensureCapabilitySet(tx *gorm.DB, typ, name, description, icon string, sortOrder int) (*CapabilitySet, error) {
	if !validCapabilityTypes[typ] {
		return nil, ErrInvalidCapabilityType
	}
	var set CapabilitySet
	err := tx.Where("type = ? AND name = ?", typ, name).First(&set).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		set = CapabilitySet{Type: typ, Name: name, Description: description, Icon: icon, Sort: sortOrder, IsActive: true}
		return &set, tx.Create(&set).Error
	}
	if err != nil {
		return nil, err
	}
	updates := map[string]any{"is_active": true}
	if set.Description == "" && description != "" {
		updates["description"] = description
	}
	if set.Icon == "" && icon != "" {
		updates["icon"] = icon
	}
	if set.Sort != sortOrder {
		updates["sort"] = sortOrder
	}
	if len(updates) > 1 || !set.IsActive || set.Sort != sortOrder {
		if err := tx.Model(&set).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return &set, nil
}

func ensureCapabilitySetItems(tx *gorm.DB, setID uint, itemIDs []uint) error {
	for idx, itemID := range uniqueUintSlice(itemIDs) {
		item := CapabilitySetItem{SetID: setID, ItemID: itemID, Sort: idx}
		if err := tx.FirstOrCreate(&item, CapabilitySetItem{SetID: setID, ItemID: itemID}).Error; err != nil {
			return err
		}
	}
	return nil
}

func capabilitySetItemsForSet(db *gorm.DB, set CapabilitySet) ([]CapabilitySetItemResponse, error) {
	var memberships []CapabilitySetItem
	if err := db.Where("set_id = ?", set.ID).Order("sort ASC, item_id ASC").Find(&memberships).Error; err != nil {
		return nil, err
	}
	items := make([]CapabilitySetItemResponse, 0, len(memberships))
	for _, membership := range memberships {
		item, ok, err := capabilityItemResponse(db, set.Type, membership.ItemID)
		if err != nil {
			return nil, err
		}
		if ok {
			items = append(items, item)
		}
	}
	return items, nil
}

func capabilityItemResponse(db *gorm.DB, typ string, itemID uint) (CapabilitySetItemResponse, bool, error) {
	switch typ {
	case CapabilityTypeTool:
		var item Tool
		if err := db.First(&item, itemID).Error; err != nil {
			return CapabilitySetItemResponse{}, false, ignoreNotFound(err)
		}
		availability := classifyToolAvailability(item, true)
		return CapabilitySetItemResponse{
			ID:                 item.ID,
			Name:               item.Name,
			DisplayName:        item.DisplayName,
			Description:        item.Description,
			IsActive:           item.IsActive,
			IsExecutable:       availability.IsExecutable,
			AvailabilityStatus: availability.Status,
			AvailabilityReason: availability.Reason,
		}, true, nil
	case CapabilityTypeMCP:
		var item MCPServer
		if err := db.First(&item, itemID).Error; err != nil {
			return CapabilitySetItemResponse{}, false, ignoreNotFound(err)
		}
		return CapabilitySetItemResponse{ID: item.ID, Name: item.Name, Description: item.Description, IsActive: item.IsActive}, true, nil
	case CapabilityTypeSkill:
		var item Skill
		if err := db.First(&item, itemID).Error; err != nil {
			return CapabilitySetItemResponse{}, false, ignoreNotFound(err)
		}
		return CapabilitySetItemResponse{ID: item.ID, Name: item.Name, DisplayName: item.DisplayName, Description: item.Description, IsActive: item.IsActive}, true, nil
	case CapabilityTypeKnowledgeBase, CapabilityTypeKnowledgeGraph:
		var item KnowledgeAsset
		if err := db.First(&item, itemID).Error; err != nil {
			return CapabilitySetItemResponse{}, false, ignoreNotFound(err)
		}
		return CapabilitySetItemResponse{ID: item.ID, Name: item.Name, Description: item.Description, IsActive: item.Status != AssetStatusError}, true, nil
	default:
		return CapabilitySetItemResponse{}, false, ErrInvalidCapabilityType
	}
}

func ignoreNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}

func selectedCapabilityItemIDsByType(db *gorm.DB, agentID uint, typ string) ([]uint, error) {
	var ids []uint
	err := db.Table("ai_agent_capability_set_items").
		Select("DISTINCT ai_agent_capability_set_items.item_id").
		Joins("JOIN ai_capability_sets ON ai_capability_sets.id = ai_agent_capability_set_items.set_id").
		Joins("JOIN ai_capability_set_items ON ai_capability_set_items.set_id = ai_agent_capability_set_items.set_id AND ai_capability_set_items.item_id = ai_agent_capability_set_items.item_id").
		Where("ai_agent_capability_set_items.agent_id = ? AND ai_agent_capability_set_items.enabled = ? AND ai_capability_sets.type = ? AND ai_capability_sets.is_active = ?", agentID, true, typ, true).
		Order("ai_agent_capability_set_items.item_id ASC").
		Pluck("ai_agent_capability_set_items.item_id", &ids).Error
	return ids, err
}

func agentHasCapabilitySetBindings(db *gorm.DB, agentID uint) (bool, error) {
	var count int64
	if err := db.Model(&AgentCapabilitySet{}).Where("agent_id = ?", agentID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func legacyFallbackOrSelectedIDs(db *gorm.DB, agentID uint, typ string, fallback func() ([]uint, error)) ([]uint, error) {
	hasSetBindings, err := agentHasCapabilitySetBindings(db, agentID)
	if err != nil {
		return nil, err
	}
	if !hasSetBindings {
		return fallback()
	}
	return selectedCapabilityItemIDsByType(db, agentID, typ)
}

func validateCapabilitySetBindings(db *gorm.DB, bindings []AgentCapabilitySetBinding) ([]AgentCapabilitySetBinding, map[string][]uint, error) {
	if len(bindings) == 0 {
		return nil, emptyDerivedBindings(), nil
	}
	if err := ensureDefaultCapabilitySets(db); err != nil {
		return nil, nil, err
	}
	normalized := make([]AgentCapabilitySetBinding, 0, len(bindings))
	derived := emptyDerivedBindings()
	seenSets := map[uint]int{}
	for _, binding := range bindings {
		if binding.SetID == 0 {
			return nil, nil, ErrInvalidBinding
		}
		var set CapabilitySet
		if err := db.First(&set, binding.SetID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil, ErrInvalidBinding
			}
			return nil, nil, err
		}
		if !set.IsActive || !validCapabilityTypes[set.Type] {
			return nil, nil, ErrInvalidBinding
		}
		itemIDs, err := uniqueUintIDs(binding.ItemIDs)
		if err != nil {
			return nil, nil, err
		}
		if err := validateCapabilitySetItems(db, set, itemIDs); err != nil {
			return nil, nil, err
		}
		if existingIdx, ok := seenSets[set.ID]; ok {
			merged := append(normalized[existingIdx].ItemIDs, itemIDs...)
			normalized[existingIdx].ItemIDs = uniqueUintSlice(merged)
		} else {
			seenSets[set.ID] = len(normalized)
			normalized = append(normalized, AgentCapabilitySetBinding{SetID: set.ID, ItemIDs: itemIDs})
		}
		derived[set.Type] = append(derived[set.Type], itemIDs...)
	}
	for typ, ids := range derived {
		derived[typ] = uniqueUintSlice(ids)
	}
	return normalized, derived, nil
}

func validateCapabilitySetItems(db *gorm.DB, set CapabilitySet, itemIDs []uint) error {
	if len(itemIDs) == 0 {
		return nil
	}
	var count int64
	if err := db.Model(&CapabilitySetItem{}).Where("set_id = ? AND item_id IN ?", set.ID, itemIDs).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(itemIDs)) {
		return ErrInvalidBinding
	}
	switch set.Type {
	case CapabilityTypeTool:
		if err := ensureIDsExist(db, &Tool{}, itemIDs, ""); err != nil {
			return err
		}
		var tools []Tool
		if err := db.Where("id IN ?", itemIDs).Find(&tools).Error; err != nil {
			return err
		}
		for _, tool := range tools {
			if !classifyToolAvailability(tool, true).IsExecutable {
				return ErrInvalidBinding
			}
		}
		return nil
	case CapabilityTypeMCP:
		return ensureIDsExist(db, &MCPServer{}, itemIDs, "")
	case CapabilityTypeSkill:
		return ensureIDsExist(db, &Skill{}, itemIDs, "")
	case CapabilityTypeKnowledgeBase:
		return ensureIDsExist(db, &KnowledgeAsset{}, itemIDs, AssetCategoryKB)
	case CapabilityTypeKnowledgeGraph:
		return ensureIDsExist(db, &KnowledgeAsset{}, itemIDs, AssetCategoryKG)
	default:
		return ErrInvalidCapabilityType
	}
}

func capabilitySetBindingsFromFlat(db *gorm.DB, bindings AgentBindings) ([]AgentCapabilitySetBinding, error) {
	if err := ensureDefaultCapabilitySets(db); err != nil {
		return nil, err
	}
	bySet := map[uint][]uint{}
	add := func(typ string, ids []uint) error {
		for _, itemID := range ids {
			setID, err := firstCapabilitySetIDForItem(db, typ, itemID)
			if err != nil {
				return err
			}
			if setID == 0 {
				return fmt.Errorf("%w: missing capability set for %s item %d", ErrInvalidBinding, typ, itemID)
			}
			bySet[setID] = append(bySet[setID], itemID)
		}
		return nil
	}
	if err := add(CapabilityTypeTool, bindings.ToolIDs); err != nil {
		return nil, err
	}
	if err := add(CapabilityTypeSkill, bindings.SkillIDs); err != nil {
		return nil, err
	}
	if err := add(CapabilityTypeMCP, bindings.MCPServerIDs); err != nil {
		return nil, err
	}
	if err := add(CapabilityTypeKnowledgeBase, bindings.KnowledgeBaseIDs); err != nil {
		return nil, err
	}
	if err := add(CapabilityTypeKnowledgeGraph, bindings.KnowledgeGraphIDs); err != nil {
		return nil, err
	}
	setIDs := make([]int, 0, len(bySet))
	for setID := range bySet {
		setIDs = append(setIDs, int(setID))
	}
	sort.Ints(setIDs)
	result := make([]AgentCapabilitySetBinding, 0, len(setIDs))
	for _, setID := range setIDs {
		result = append(result, AgentCapabilitySetBinding{SetID: uint(setID), ItemIDs: uniqueUintSlice(bySet[uint(setID)])})
	}
	return result, nil
}

func firstCapabilitySetIDForItem(db *gorm.DB, typ string, itemID uint) (uint, error) {
	var row CapabilitySetItem
	err := db.Table("ai_capability_set_items").
		Select("ai_capability_set_items.*").
		Joins("JOIN ai_capability_sets ON ai_capability_sets.id = ai_capability_set_items.set_id").
		Where("ai_capability_sets.type = ? AND ai_capability_sets.is_active = ? AND ai_capability_set_items.item_id = ?", typ, true, itemID).
		Order("ai_capability_sets.sort ASC, ai_capability_sets.id ASC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return row.SetID, err
}

func replaceCapabilityBindingsInTx(tx *gorm.DB, agentID uint, bindings []AgentCapabilitySetBinding) error {
	if err := tx.Where("agent_id = ?", agentID).Delete(&AgentCapabilitySetItem{}).Error; err != nil {
		return err
	}
	if err := tx.Where("agent_id = ?", agentID).Delete(&AgentCapabilitySet{}).Error; err != nil {
		return err
	}
	for _, binding := range bindings {
		if err := tx.Create(&AgentCapabilitySet{AgentID: agentID, SetID: binding.SetID}).Error; err != nil {
			return err
		}
		for _, itemID := range binding.ItemIDs {
			item := AgentCapabilitySetItem{AgentID: agentID, SetID: binding.SetID, ItemID: itemID, Enabled: true}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func getAgentCapabilitySetBindings(db *gorm.DB, agentID uint) ([]AgentCapabilitySetBinding, error) {
	var sets []AgentCapabilitySet
	if err := db.Where("agent_id = ?", agentID).Find(&sets).Error; err != nil {
		return nil, err
	}
	result := make([]AgentCapabilitySetBinding, 0, len(sets))
	for _, set := range sets {
		var ids []uint
		if err := db.Model(&AgentCapabilitySetItem{}).
			Where("agent_id = ? AND set_id = ? AND enabled = ?", agentID, set.SetID, true).
			Order("item_id ASC").
			Pluck("item_id", &ids).Error; err != nil {
			return nil, err
		}
		result = append(result, AgentCapabilitySetBinding{SetID: set.SetID, ItemIDs: ids})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SetID < result[j].SetID })
	return result, nil
}

func migrateAgentCapabilitySetBindings(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := ensureDefaultCapabilitySets(tx); err != nil {
			return err
		}
		var agents []Agent
		if err := tx.Find(&agents).Error; err != nil {
			return err
		}
		repo := &AgentRepo{db: &database.DB{DB: tx}}
		for _, agent := range agents {
			hasSetBindings, err := agentHasCapabilitySetBindings(tx, agent.ID)
			if err != nil {
				return err
			}
			if hasSetBindings {
				continue
			}
			flat, err := repo.getLegacyBindings(agent.ID)
			if err != nil {
				return err
			}
			if !flat.hasAnyFlatBinding() {
				continue
			}
			setBindings, err := capabilitySetBindingsFromFlat(tx, flat)
			if err != nil {
				return err
			}
			if err := replaceCapabilityBindingsInTx(tx, agent.ID, setBindings); err != nil {
				return err
			}
		}
		return nil
	})
}

func emptyDerivedBindings() map[string][]uint {
	return map[string][]uint{
		CapabilityTypeTool:           nil,
		CapabilityTypeMCP:            nil,
		CapabilityTypeSkill:          nil,
		CapabilityTypeKnowledgeBase:  nil,
		CapabilityTypeKnowledgeGraph: nil,
	}
}

func uniqueUintSlice(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	unique := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	sort.Slice(unique, func(i, j int) bool { return unique[i] < unique[j] })
	return unique
}

func capabilityTypeFromQuery(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !validCapabilityTypes[value] {
		return "", fmt.Errorf("%w: %s", ErrInvalidCapabilityType, value)
	}
	return value, nil
}

func parseCapabilitySetID(c *gin.Context) (uint, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		return 0, ErrCapabilitySetNotFound
	}
	return uint(id), nil
}
