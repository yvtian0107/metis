import { describe, expect, test } from "bun:test"
import { itsmQueryKeys } from "./query-keys"

describe("itsmQueryKeys", () => {
  test("keeps service list keys stable and parameterized", () => {
    expect(itsmQueryKeys.services.list({ page: 2, pageSize: 24, rootCatalogId: 7 })).toEqual([
      "itsm",
      "services",
      "list",
      { page: 2, pageSize: 24, rootCatalogId: 7 },
    ])
  })

  test("separates service detail, actions, catalogs and catalog counts", () => {
    expect(itsmQueryKeys.services.detail(5)).toEqual(["itsm", "services", "detail", 5])
    expect(itsmQueryKeys.services.actions(5)).toEqual(["itsm", "services", "actions", 5])
    expect(itsmQueryKeys.catalogs.tree()).toEqual(["itsm", "catalogs", "tree"])
    expect(itsmQueryKeys.catalogs.serviceCounts()).toEqual(["itsm", "catalogs", "service-counts"])
  })

  test("keeps priority and SLA rule keys shared across pages", () => {
    expect(itsmQueryKeys.priorities.all).toEqual(["itsm", "priorities"])
    expect(itsmQueryKeys.sla.all).toEqual(["itsm", "sla"])
    expect(itsmQueryKeys.sla.escalations(7)).toEqual(["itsm", "sla", 7, "escalations"])
    expect(itsmQueryKeys.sla.notificationChannels).toEqual(["itsm", "sla", "notification-channels"])
  })
})
