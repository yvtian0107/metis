import { describe, expect, test } from "bun:test"
import type { CatalogItem, CatalogServiceCounts } from "../../api"
import {
  getCatalogSelection,
  getCreateServiceCatalogDefault,
  getDisplayedCatalogCount,
  shouldResetServiceForm,
} from "./service-catalog-state"

const rootWithChild: CatalogItem = {
  id: 1,
  parentId: null,
  name: "Root",
  code: "root",
  description: "",
  icon: "",
  sortOrder: 1,
  isActive: true,
  createdAt: "",
  updatedAt: "",
  children: [
    {
      id: 2,
      parentId: 1,
      name: "Child",
      code: "child",
      description: "",
      icon: "",
      sortOrder: 1,
      isActive: true,
      createdAt: "",
      updatedAt: "",
    },
  ],
}

const leafRoot: CatalogItem = {
  id: 3,
  parentId: null,
  name: "Leaf Root",
  code: "leaf-root",
  description: "",
  icon: "",
  sortOrder: 2,
  isActive: true,
  createdAt: "",
  updatedAt: "",
  children: [],
}

const counts: CatalogServiceCounts = {
  total: 9,
  byCatalogId: { 1: 1, 2: 3, 3: 5 },
  byRootCatalogId: { 1: 4, 3: 5 },
}

describe("service catalog state", () => {
  test("selecting a root catalog filters by rootCatalogId and includes direct root services", () => {
    expect(getCatalogSelection([rootWithChild, leafRoot], 1)).toEqual({ rootCatalogId: 1 })
  })

  test("selecting a child catalog filters by exact catalogId", () => {
    expect(getCatalogSelection([rootWithChild, leafRoot], 2)).toEqual({ catalogId: 2 })
  })

  test("create service does not prefill a parent root that has children", () => {
    expect(getCreateServiceCatalogDefault([rootWithChild, leafRoot], 1)).toBe(0)
    expect(getCreateServiceCatalogDefault([rootWithChild, leafRoot], 2)).toBe(2)
    expect(getCreateServiceCatalogDefault([rootWithChild, leafRoot], 3)).toBe(3)
  })

  test("catalog nav shows direct child counts and aggregated root counts", () => {
    expect(getDisplayedCatalogCount(counts, null)).toBe(9)
    expect(getDisplayedCatalogCount(counts, 1, "root")).toBe(4)
    expect(getDisplayedCatalogCount(counts, 2, "child")).toBe(3)
    expect(getDisplayedCatalogCount(counts, 999, "child")).toBe(0)
  })

  test("service form resets only when switching service or when not dirty", () => {
    expect(shouldResetServiceForm({ previousServiceId: 1, nextServiceId: 2, isDirty: true })).toBe(true)
    expect(shouldResetServiceForm({ previousServiceId: 1, nextServiceId: 1, isDirty: false })).toBe(true)
    expect(shouldResetServiceForm({ previousServiceId: 1, nextServiceId: 1, isDirty: true })).toBe(false)
  })
})
