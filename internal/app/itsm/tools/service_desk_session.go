package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"metis/internal/app/itsm/contract"
	"strings"
)

type ServiceDeskSession struct {
	op    ServiceDeskOperator
	store StateStore
}

func NewServiceDeskSession(op ServiceDeskOperator, store StateStore) *ServiceDeskSession {
	return &ServiceDeskSession{op: op, store: store}
}

func clearPendingNextRequiredTool(state *ServiceDeskState) {
	if state == nil {
		return
	}
	state.PendingNextRequiredTool = ""
}

func (s *ServiceDeskSession) state(sessionID uint) (*ServiceDeskState, error) {
	state, err := s.store.GetState(sessionID)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = defaultState()
	}
	return state, nil
}

func (s *ServiceDeskSession) save(sessionID uint, state *ServiceDeskState) error {
	return s.store.SaveState(sessionID, state)
}

func (s *ServiceDeskSession) StateView(sessionID uint) (*ServiceDeskCommandResult, error) {
	state, err := s.state(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	return s.result(true, state, nil), nil
}

func (s *ServiceDeskSession) CurrentContext(sessionID uint) (*ServiceDeskCommandResult, error) {
	state, err := s.state(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	payload := statePayload(state)
	return s.result(true, state, payload), nil
}

func (s *ServiceDeskSession) MatchServices(ctx context.Context, sessionID uint, query string) (*ServiceDeskCommandResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	requestText := originalRequestText(ctx, query)
	state, err := s.state(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	if shouldReuseLoadedService(state, query) {
		payload := map[string]any{
			"ok":                 true,
			"already_loaded":     true,
			"service_locked":     true,
			"loaded_service_id":  state.LoadedServiceID,
			"service_version_id": state.ServiceVersionID,
			"request_text":       state.RequestText,
			"prefill_form_data":  state.PrefillFormData,
			"state_stage":        state.Stage,
		}
		return s.result(true, state, payload), nil
	}

	matches, decision, err := s.op.MatchServices(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("match services: %w", err)
	}

	confirmationRequired := false
	var selectedServiceID uint
	switch decision.Kind {
	case MatchDecisionSelectService:
		selectedServiceID = decision.SelectedServiceID
		if selectedServiceID == 0 && len(matches) == 1 {
			selectedServiceID = matches[0].ID
		}
		if selectedServiceID == 0 {
			return nil, fmt.Errorf("select_service decision missing selected_service_id")
		}
		var selectedMatch *ServiceMatch
		for i := range matches {
			if matches[i].ID == selectedServiceID {
				selectedMatch = &matches[i]
				break
			}
		}
		if selectedMatch == nil {
			return nil, fmt.Errorf("select_service decision service_id %d not found in matches", selectedServiceID)
		}
		matches = []ServiceMatch{*selectedMatch}
	case MatchDecisionNeedClarification:
		confirmationRequired = true
	case MatchDecisionNoMatch:
		matches = nil
	default:
		return nil, fmt.Errorf("unknown service match decision: %s", decision.Kind)
	}

	candidateIDs := make([]uint, len(matches))
	for i, m := range matches {
		candidateIDs[i] = m.ID
	}

	var topMatchID uint
	if len(matches) > 0 {
		topMatchID = matches[0].ID
	}

	if err := state.TransitionTo("candidates_ready"); err != nil {
		return nil, err
	}
	state.CandidateServiceIDs = candidateIDs
	state.TopMatchServiceID = topMatchID
	state.ConfirmationRequired = confirmationRequired
	state.ConfirmedServiceID = selectedServiceID
	state.LoadedServiceID = 0
	state.ServiceVersionID = 0
	state.ServiceVersionHash = ""
	state.RequestText = requestText
	state.PrefillFormData = nil
	clearPendingNextRequiredTool(state)
	if err := s.save(sessionID, state); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}

	payload := map[string]any{
		"query":                  query,
		"matches":                matches,
		"confirmation_required":  confirmationRequired,
		"selected_service_id":    selectedServiceID,
		"service_locked":         selectedServiceID > 0 && !confirmationRequired,
		"next_required_tool":     serviceMatchNextRequiredTool(selectedServiceID, confirmationRequired, len(matches)),
		"clarification_question": decision.ClarificationQuestion,
	}
	return s.result(true, state, payload), nil
}

func (s *ServiceDeskSession) ConfirmService(sessionID uint, requestedServiceID uint) (*ServiceDeskCommandResult, error) {
	if requestedServiceID == 0 {
		return nil, fmt.Errorf("service_id is required")
	}
	state, err := s.state(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	if state.Stage != "candidates_ready" {
		return nil, fmt.Errorf("当前阶段不允许确认服务，请先调用 service_match")
	}
	resolvedServiceID, resolvedFrom, err := resolveServiceID(state, requestedServiceID)
	if err != nil || resolvedFrom == "loaded_service" {
		return nil, fmt.Errorf("service_id %d 不在候选列表中，也不是合法候选序号", requestedServiceID)
	}
	state.ConfirmedServiceID = resolvedServiceID
	if err := state.TransitionTo("service_selected"); err != nil {
		return nil, err
	}
	clearPendingNextRequiredTool(state)
	if err := s.save(sessionID, state); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}
	payload := map[string]any{
		"ok":                   true,
		"service_id":           resolvedServiceID,
		"confirmed_service_id": resolvedServiceID,
		"requested_service_id": requestedServiceID,
		"resolved_from":        resolvedFrom,
		"next_required_tool":   "itsm.service_load",
	}
	return s.result(true, state, payload), nil
}

func (s *ServiceDeskSession) LoadService(sessionID uint, requestedServiceID uint) (*ServiceDeskCommandResult, error) {
	if requestedServiceID == 0 {
		return nil, fmt.Errorf("service_id is required")
	}
	state, err := s.state(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	if len(state.CandidateServiceIDs) == 0 && state.ConfirmedServiceID == 0 && state.LoadedServiceID == 0 {
		return nil, fmt.Errorf("请先调用 service_match 匹配服务，不能凭空加载服务")
	}
	if state.ConfirmationRequired && state.ConfirmedServiceID == 0 {
		return nil, fmt.Errorf("请先调用 service_confirm")
	}

	resolvedServiceID, resolvedFrom, err := resolveServiceID(state, requestedServiceID)
	if err != nil {
		return nil, err
	}

	detail, err := s.op.LoadService(resolvedServiceID)
	if err != nil {
		return nil, fmt.Errorf("load service: %w", err)
	}
	detail.RequestedID = requestedServiceID
	detail.ResolvedFrom = resolvedFrom
	detail.PrefillSuggestions = buildPrefillSuggestions(state.RequestText, detail.FormFields)
	detail.FieldCollection = buildFieldCollection(detail.FormFields, detail.PrefillSuggestions, detail.RoutingFieldHint)
	syncConversationProgress(state, detail.FieldCollection.MissingRequiredFields)

	if state.LoadedServiceID == resolvedServiceID &&
		(state.Stage == "service_loaded" || state.Stage == "awaiting_confirmation" || state.Stage == "confirmed") {
		if state.FieldsHash != detail.FieldsHash || state.ServiceVersionID != detail.ServiceVersionID {
			state.FieldsHash = detail.FieldsHash
			state.ServiceVersionID = detail.ServiceVersionID
			state.ServiceVersionHash = detail.ServiceVersionHash
			state.ConfirmedDraftVersion = 0
		}
		state.PrefillFormData = detail.PrefillSuggestions
		clearPendingNextRequiredTool(state)
		if err := s.save(sessionID, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}
		return s.result(true, state, serviceDetailPayload(detail)), nil
	}

	if state.LoadedServiceID != resolvedServiceID {
		state.DraftSummary = ""
		state.DraftFormData = nil
		state.DraftVersion = 0
		state.ConfirmedDraftVersion = 0
	}

	state.LoadedServiceID = resolvedServiceID
	state.ServiceVersionID = detail.ServiceVersionID
	state.ServiceVersionHash = detail.ServiceVersionHash
	state.FieldsHash = detail.FieldsHash
	state.PrefillFormData = detail.PrefillSuggestions
	if err := state.TransitionTo("service_loaded"); err != nil {
		return nil, err
	}
	clearPendingNextRequiredTool(state)
	if err := s.save(sessionID, state); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}
	return s.result(true, state, serviceDetailPayload(detail)), nil
}

func (s *ServiceDeskSession) Reset(sessionID uint) (*ServiceDeskCommandResult, error) {
	state := defaultState()
	clearPendingNextRequiredTool(state)
	if err := s.save(sessionID, state); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}
	return s.result(true, state, map[string]any{"ok": true, "message": "已就绪，请描述您的需求"}), nil
}

func (s *ServiceDeskSession) PrepareDraft(sessionID uint, summary string, formData map[string]any) (*ServiceDeskCommandResult, error) {
	state, err := s.state(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	if state.LoadedServiceID == 0 {
		return nil, fmt.Errorf("请先调用 service_load 加载服务")
	}

	detail, err := s.op.LoadService(state.LoadedServiceID)
	if err != nil {
		return nil, fmt.Errorf("load service for validation: %w", err)
	}
	if err := ensureLoadedSnapshotMatches(state, detail); err != nil {
		return nil, err
	}
	if detail.EngineType == "smart" && len(detail.FormFields) == 0 {
		state.PendingNextRequiredTool = "generate_reference_path"
		if err := s.save(sessionID, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}
		payload := map[string]any{
			"ok":                      false,
			"ready_for_confirmation":  false,
			"next_required_tool":      "generate_reference_path",
			"recommended_next_step":   "generate_reference_path",
			"missing_required_fields": []FieldCollectionItem{},
			"summary":                 summary,
			"form_data":               map[string]any{},
			"warnings": []DraftWarning{{
				Type:    "missing_form_schema",
				Field:   "intake_form_schema",
				Message: "申请确认表单未生成，请由管理员先生成参考路径",
			}},
		}
		return s.result(false, state, payload), nil
	}

	// Guard: agent retrying draft_prepare without collecting from user.
	// If we already flagged collect_missing_fields and the new form_data is still
	// empty/nil, the agent is looping without making progress. Return an explicit
	// error so the LLM stops retrying and asks the user instead.
	if pendingNextRequiredTool(state) == "collect_missing_fields" && len(formData) == 0 {
		return nil, fmt.Errorf("draft_prepare 已阻塞（next_required_tool=collect_missing_fields），禁止在未收到用户新信息的情况下以空 form_data 重复调用；请先向用户逐项追问 missing_required_fields 中的缺口，待用户回复后再重新调用。")
	}

	formData = normalizeFormDataKeys(formData, detail.FormFields)
	formData = mergePrefillFormData(formData, state.PrefillFormData)
	validationContext := buildValidationContext(state.RequestText, summary)
	formData = canonicalizeTimeSemanticFields(detail, validationContext, formData)
	warnings, missingRequired, blocking := validateDraftData(detail, formData, validationContext)
	timeWarnings, timeMissing, timeBlocking := validateDraftTimeSource(detail, validationContext, formData)
	warnings = append(warnings, timeWarnings...)
	missingRequired = append(missingRequired, timeMissing...)
	blocking = blocking || timeBlocking
	syncConversationProgress(state, missingRequired)

	if blocking {
		state.PendingNextRequiredTool = "collect_missing_fields"
		if err := s.save(sessionID, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}
		payload := map[string]any{
			"ok":                      false,
			"ready_for_confirmation":  false,
			"next_required_tool":      "collect_missing_fields",
			"recommended_next_step":   "ask_missing_fields",
			"missing_required_fields": missingRequired,
			"summary":                 summary,
			"form_data":               formData,
			"warnings":                warnings,
		}
		result := s.result(false, state, payload)
		result.Warnings = warnings
		result.MissingFields = missingRequired
		return result, nil
	}

	contentChanged := state.DraftSummary != summary || hashFormData(state.DraftFormData) != hashFormData(formData)
	if contentChanged {
		state.DraftVersion++
		state.ConfirmedDraftVersion = 0
	}
	state.DraftSummary = summary
	state.DraftFormData = formData
	if err := state.TransitionTo("awaiting_confirmation"); err != nil {
		return nil, err
	}
	clearPendingNextRequiredTool(state)
	if err := s.save(sessionID, state); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}

	surface := buildReadyDraftSurface(detail, state, summary, formData)
	payload := map[string]any{
		"ok":                      true,
		"ready_for_confirmation":  true,
		"next_required_tool":      "itsm.draft_confirm",
		"recommended_next_step":   "show_draft_for_confirmation",
		"missing_required_fields": []FieldCollectionItem{},
		"draft_version":           state.DraftVersion,
		"service_id":              detail.ServiceID,
		"service_version_id":      detail.ServiceVersionID,
		"service_name":            detail.Name,
		"service_engine_type":     detail.EngineType,
		"summary":                 summary,
		"form_data":               formData,
		"form_schema":             detail.FormSchema,
		"warnings":                warnings,
		"surface":                 surface,
	}
	result := s.result(true, state, payload)
	result.Surface = surface
	result.Warnings = warnings
	return result, nil
}

func (s *ServiceDeskSession) ConfirmDraft(sessionID uint) (*ServiceDeskCommandResult, error) {
	state, err := s.state(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	if state.Stage != "awaiting_confirmation" {
		return nil, fmt.Errorf("当前阶段不允许确认草稿，请先调用 draft_prepare")
	}
	if state.LoadedServiceID > 0 {
		detail, err := s.op.LoadService(state.LoadedServiceID)
		if err != nil {
			return nil, fmt.Errorf("load service for hash check: %w", err)
		}
		if err := ensureLoadedSnapshotMatches(state, detail); err != nil {
			return nil, err
		}
		validationContext := buildValidationContext(state.RequestText, state.DraftSummary)
		state.DraftFormData = canonicalizeTimeSemanticFields(detail, validationContext, state.DraftFormData)
		warnings, _, blocking := validateDraftData(detail, state.DraftFormData, validationContext)
		timeWarnings, _, timeBlocking := validateDraftTimeSource(detail, validationContext, state.DraftFormData)
		warnings = append(warnings, timeWarnings...)
		blocking = blocking || timeBlocking
		if blocking {
			if len(warnings) > 0 {
				return nil, fmt.Errorf("草稿表单校验失败：%s", warnings[0].Message)
			}
			return nil, fmt.Errorf("草稿表单校验失败，请重新调用 draft_prepare")
		}
	}
	state.ConfirmedDraftVersion = state.DraftVersion
	if err := state.TransitionTo("confirmed"); err != nil {
		return nil, err
	}
	clearPendingNextRequiredTool(state)
	if err := s.save(sessionID, state); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}
	payload := map[string]any{
		"ok":                      true,
		"draft_version":           state.DraftVersion,
		"confirmed_draft_version": state.ConfirmedDraftVersion,
	}
	return s.result(true, state, payload), nil
}

func (s *ServiceDeskSession) ValidateParticipants(sessionID uint, requestedServiceID uint, formData map[string]any) (*ServiceDeskCommandResult, error) {
	if requestedServiceID == 0 {
		return nil, fmt.Errorf("service_id is required")
	}
	state, err := s.state(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	resolvedServiceID, _, err := resolveServiceID(state, requestedServiceID)
	if err != nil {
		return nil, err
	}
	if len(formData) == 0 && len(state.DraftFormData) > 0 {
		formData = state.DraftFormData
	}
	result, err := s.op.ValidateParticipants(resolvedServiceID, formData)
	if err != nil {
		return nil, fmt.Errorf("validate participants: %w", err)
	}
	payload := map[string]any{}
	if result != nil {
		payload["ok"] = result.OK
		payload["failure_reason"] = result.FailureReason
		payload["node_label"] = result.NodeLabel
		payload["guidance"] = result.Guidance
	}
	return s.result(result == nil || result.OK, state, payload), nil
}

func (s *ServiceDeskSession) CreateTicket(sessionID uint, userID uint, requestedServiceID uint, summary string, formData map[string]any) (*ServiceDeskCommandResult, error) {
	if requestedServiceID == 0 {
		return nil, fmt.Errorf("service_id is required")
	}
	state, err := s.state(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	resolvedServiceID, _, err := resolveServiceID(state, requestedServiceID)
	if err != nil {
		return nil, err
	}
	if state.LoadedServiceID != resolvedServiceID {
		return nil, fmt.Errorf("service_id %d 与已加载的服务 %d 不一致，请先调用 service_load", requestedServiceID, state.LoadedServiceID)
	}
	if state.ConfirmedDraftVersion == 0 {
		return nil, fmt.Errorf("草稿未确认，请先调用 draft_confirm")
	}
	if state.ConfirmedDraftVersion != state.DraftVersion {
		return nil, fmt.Errorf("草稿已变更（当前版本 %d，已确认版本 %d），请重新调用 draft_confirm", state.DraftVersion, state.ConfirmedDraftVersion)
	}
	if state.DraftSummary != "" {
		summary = state.DraftSummary
	}
	if len(state.DraftFormData) > 0 {
		formData = state.DraftFormData
	}
	detail, err := s.op.LoadService(state.LoadedServiceID)
	if err != nil {
		return nil, fmt.Errorf("load service for create: %w", err)
	}
	if err := ensureLoadedSnapshotMatches(state, detail); err != nil {
		return nil, err
	}
	ticket, err := s.op.SubmitConfirmedDraft(userID, resolvedServiceID, state.ServiceVersionID, summary, formData, sessionID, state.DraftVersion, state.FieldsHash, requestHash(formData))
	if err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}
	resetState := defaultState()
	clearPendingNextRequiredTool(resetState)
	if err := s.save(sessionID, resetState); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}
	payload := map[string]any{
		"ok":          true,
		"ticket_id":   ticket.TicketID,
		"ticket_code": ticket.TicketCode,
		"status":      ticket.Status,
	}
	result := s.result(true, resetState, payload)
	result.Message = "工单已提交"
	result.Surface = buildSubmittedDraftSurface(detail, ticket, summary, formData)
	return result, nil
}

func (s *ServiceDeskSession) SubmitDraft(sessionID uint, userID uint, req DraftSubmitRequest) (*DraftSubmitResult, error) {
	state, err := s.state(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	if state.Stage != "awaiting_confirmation" {
		return nil, fmt.Errorf("当前阶段不允许提交草稿，请先完成草稿整理")
	}
	if state.LoadedServiceID == 0 {
		return nil, fmt.Errorf("请先加载服务")
	}
	if req.DraftVersion == 0 || req.DraftVersion != state.DraftVersion {
		return nil, fmt.Errorf("草稿已变更（当前版本 %d，提交版本 %d），请重新确认表单", state.DraftVersion, req.DraftVersion)
	}
	detail, err := s.op.LoadService(state.LoadedServiceID)
	if err != nil {
		return nil, fmt.Errorf("load service for submit: %w", err)
	}
	if detail.EngineType != "smart" {
		return nil, fmt.Errorf("仅支持 Agentic 服务提交")
	}
	if err := ensureLoadedSnapshotMatches(state, detail); err != nil {
		return nil, err
	}

	formData := normalizeFormDataKeys(req.FormData, detail.FormFields)
	if len(formData) == 0 && len(state.DraftFormData) > 0 {
		formData = normalizeFormDataKeys(state.DraftFormData, detail.FormFields)
	}
	formData = mergePrefillFormData(formData, state.PrefillFormData)
	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		summary = state.DraftSummary
	}
	validationContext := buildValidationContext(state.RequestText, summary)
	formData = canonicalizeTimeSemanticFields(detail, validationContext, formData)
	warnings, missingRequired, blocking := validateDraftData(detail, formData, validationContext)
	timeWarnings, timeMissing, timeBlocking := validateDraftTimeSource(detail, validationContext, formData)
	warnings = append(warnings, timeWarnings...)
	missingRequired = append(missingRequired, timeMissing...)
	blocking = blocking || timeBlocking
	if blocking {
		return &DraftSubmitResult{
			OK:                 false,
			Message:            "表单还有必填项或无效值，请补充后再提交。",
			NextExpectedAction: NextExpectedAction(state),
			Warnings:           warnings,
			MissingFields:      missingRequired,
			State:              state,
		}, nil
	}
	validation, err := s.op.ValidateParticipants(state.LoadedServiceID, formData)
	if err != nil {
		return nil, fmt.Errorf("validate participants: %w", err)
	}
	if validation != nil && !validation.OK {
		return &DraftSubmitResult{
			OK:                 false,
			Message:            "参与者预检失败，工单未创建。",
			NextExpectedAction: NextExpectedAction(state),
			FailureReason:      validation.FailureReason,
			NodeLabel:          validation.NodeLabel,
			Guidance:           validation.Guidance,
			Warnings:           warnings,
			State:              state,
		}, nil
	}
	state.DraftSummary = summary
	state.DraftFormData = formData
	state.ConfirmedDraftVersion = state.DraftVersion

	ticket, err := s.op.SubmitConfirmedDraft(userID, state.LoadedServiceID, state.ServiceVersionID, summary, formData, sessionID, state.DraftVersion, state.FieldsHash, requestHash(formData))
	if err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}
	resetState := defaultState()
	clearPendingNextRequiredTool(resetState)
	if err := s.save(sessionID, resetState); err != nil {
		return nil, fmt.Errorf("save reset state: %w", err)
	}
	surface := buildSubmittedDraftSurface(detail, ticket, summary, formData)
	return &DraftSubmitResult{
		OK:                 true,
		TicketID:           ticket.TicketID,
		TicketCode:         ticket.TicketCode,
		Status:             ticket.Status,
		Message:            "工单已提交",
		NextExpectedAction: NextExpectedAction(resetState),
		Warnings:           warnings,
		State:              resetState,
		Surface:            surface,
	}, nil
}

func (s *ServiceDeskSession) result(ok bool, state *ServiceDeskState, payload map[string]any) *ServiceDeskCommandResult {
	if payload == nil {
		payload = map[string]any{}
	}
	next := NextExpectedAction(state)
	if raw, ok := payload["next_required_tool"].(string); ok && raw != "" {
		next = raw
	}
	payload["state"] = state
	payload["next_expected_action"] = next
	payload["nextExpectedAction"] = next
	if _, exists := payload["next_required_tool"]; !exists {
		payload["next_required_tool"] = next
	}
	if _, exists := payload["ok"]; !exists {
		payload["ok"] = ok
	}
	return &ServiceDeskCommandResult{
		OK:                 ok,
		NextExpectedAction: next,
		State:              state,
		Payload:            payload,
	}
}

func commandPayload(result *ServiceDeskCommandResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	payload := make(map[string]any, len(result.Payload)+6)
	for key, value := range result.Payload {
		payload[key] = value
	}
	payload["ok"] = result.OK
	payload["state"] = result.State
	payload["nextExpectedAction"] = result.NextExpectedAction
	payload["next_expected_action"] = result.NextExpectedAction
	if result.Message != "" {
		payload["message"] = result.Message
	}
	if result.Surface != nil {
		payload["surface"] = result.Surface
	}
	if result.Warnings != nil {
		payload["warnings"] = result.Warnings
	}
	if result.MissingFields != nil {
		payload["missingRequiredFields"] = result.MissingFields
		payload["missing_required_fields"] = result.MissingFields
	}
	return payload
}

func CommandPayload(result *ServiceDeskCommandResult) map[string]any {
	return commandPayload(result)
}

func statePayload(state *ServiceDeskState) map[string]any {
	return map[string]any{
		"stage":                      state.Stage,
		"candidate_service_ids":      state.CandidateServiceIDs,
		"top_match_service_id":       state.TopMatchServiceID,
		"confirmed_service_id":       state.ConfirmedServiceID,
		"confirmation_required":      state.ConfirmationRequired,
		"loaded_service_id":          state.LoadedServiceID,
		"service_version_id":         state.ServiceVersionID,
		"service_version_hash":       state.ServiceVersionHash,
		"request_text":               state.RequestText,
		"prefill_form_data":          state.PrefillFormData,
		"draft_summary":              state.DraftSummary,
		"draft_form_data":            state.DraftFormData,
		"draft_version":              state.DraftVersion,
		"confirmed_draft_version":    state.ConfirmedDraftVersion,
		"missing_fields":             state.MissingFields,
		"asked_fields":               state.AskedFields,
		"min_decision_ready":         state.MinDecisionReady,
		"pending_next_required_tool": pendingNextRequiredTool(state),
		"next_expected_action":       NextExpectedAction(state),
		"nextExpectedAction":         NextExpectedAction(state),
	}
}

func serviceDetailPayload(detail *ServiceDetail) map[string]any {
	b := mustMarshal(detail)
	var payload map[string]any
	_ = json.Unmarshal(b, &payload)
	return payload
}

func ensureLoadedSnapshotMatches(state *ServiceDeskState, detail *ServiceDetail) error {
	if detail.FieldsHash != state.FieldsHash {
		return fmt.Errorf("服务表单字段已变更，请重新整理草稿")
	}
	if state.ServiceVersionID > 0 && detail.ServiceVersionID > 0 && detail.ServiceVersionID != state.ServiceVersionID {
		return fmt.Errorf("服务定义版本已变更，请重新加载服务")
	}
	return nil
}

func buildReadyDraftSurface(detail *ServiceDetail, state *ServiceDeskState, summary string, values map[string]any) map[string]any {
	return map[string]any{
		"surfaceId":   fmt.Sprintf("itsm-draft-form-%d-%d", detail.ServiceID, state.DraftVersion),
		"surfaceType": string(contract.SurfaceTypeITSMDraftForm),
		"payload": map[string]any{
			"status":           "ready",
			"serviceId":        detail.ServiceID,
			"serviceVersionId": detail.ServiceVersionID,
			"title":            detail.Name,
			"summary":          summary,
			"schema":           detail.FormSchema,
			"values":           values,
			"draftVersion":     state.DraftVersion,
			"submitAction":     map[string]any{"kind": "rest", "method": "POST"},
		},
	}
}

func buildSubmittedDraftSurface(detail *ServiceDetail, ticket *TicketResult, summary string, values map[string]any) map[string]any {
	return map[string]any{
		"surfaceId":   fmt.Sprintf("itsm-draft-form-submitted-%d", ticket.TicketID),
		"surfaceType": string(contract.SurfaceTypeITSMDraftForm),
		"payload": map[string]any{
			"status":           "submitted",
			"serviceId":        detail.ServiceID,
			"serviceVersionId": detail.ServiceVersionID,
			"title":            detail.Name,
			"summary":          summary,
			"values":           values,
			"ticketId":         ticket.TicketID,
			"ticketCode":       ticket.TicketCode,
			"message":          "工单已提交",
		},
	}
}
