package steps

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"

	"metis/internal/app/itsm/bdd/support"
)

func RegisterAPISteps(sc *godog.ScenarioContext, ctx *support.Context) {
	sc.Given(`^API BDD 已初始化$`, func() error { return nil })
	sc.Given(`^API BDD 已准备默认 Actor$`, ctx.EnsureDefaultActors)
	sc.Given(`^API BDD 存在一张分配给 "([^"]*)" 的智能工单$`, ctx.SeedAssignedSmartTicket)

	sc.When(`^以 "([^"]*)" 身份查询待办列表$`, func(actor string) error {
		_, err := ctx.Client.Do(actor, http.MethodGet, "/api/v1/itsm/tickets/approvals/pending", nil)
		return err
	})
	sc.When(`^以 "([^"]*)" 身份认领当前待办$`, func(actor string) error {
		_, err := ctx.Client.Do(actor, http.MethodPost, fmt.Sprintf("/api/v1/itsm/tickets/%d/claim", ctx.CurrentTicketID), map[string]any{"activityId": ctx.CurrentActivityID})
		return err
	})

	sc.Then(`^API 响应状态应为 (\d+)$`, func(status int) error {
		if ctx.LastResponse == nil {
			return fmt.Errorf("no API response captured")
		}
		if ctx.LastResponse.StatusCode != status {
			return fmt.Errorf("API status = %d, want %d, body=%s", ctx.LastResponse.StatusCode, status, ctx.LastResponse.RawBody)
		}
		return nil
	})
	sc.Then(`^待办列表应包含当前工单$`, func() error { return assertPendingListContains(ctx, true) })
	sc.Then(`^待办列表不应包含当前工单$`, func() error { return assertPendingListContains(ctx, false) })
	sc.Then(`^工单状态应为 "([^"]*)"$`, func(status string) error {
		data, err := ctx.LoadTicketViaAPI("管理员")
		if err != nil {
			return err
		}
		got, err := support.DecodeField[string](data, "status")
		if err != nil {
			return err
		}
		if got != status {
			return fmt.Errorf("ticket status = %q, want %q", got, status)
		}
		return nil
	})
}

func assertPendingListContains(ctx *support.Context, want bool) error {
	data, err := ctx.LastDataObject()
	if err != nil {
		return err
	}
	rawItems, ok := data["items"]
	if !ok {
		return fmt.Errorf("pending list response missing items")
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(rawItems, &items); err != nil {
		return fmt.Errorf("decode pending items: %w", err)
	}
	found := false
	for _, item := range items {
		id, err := support.DecodeField[uint](item, "id")
		if err == nil && id == ctx.CurrentTicketID {
			found = true
			break
		}
	}
	if found != want {
		return fmt.Errorf("pending list contains current ticket = %v, want %v", found, want)
	}
	return nil
}
