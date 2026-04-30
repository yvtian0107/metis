import type { CatalogItem, CatalogServiceCounts } from "../../api"

export type CatalogSelection = {
  catalogId?: number
  rootCatalogId?: number
}

function findCatalog(catalogs: CatalogItem[], catalogId: number | null) {
  if (catalogId === null) return undefined
  for (const root of catalogs) {
    if (root.id === catalogId) return root
    const child = root.children?.find((item) => item.id === catalogId)
    if (child) return child
  }
  return undefined
}

export function getCatalogSelection(catalogs: CatalogItem[], catalogId: number | null): CatalogSelection {
  const catalog = findCatalog(catalogs, catalogId)
  if (!catalog) return {}
  if (!catalog.parentId) return { rootCatalogId: catalog.id }
  return { catalogId: catalog.id }
}

export function getCreateServiceCatalogDefault(catalogs: CatalogItem[], catalogId: number | null) {
  const catalog = findCatalog(catalogs, catalogId)
  if (!catalog) return 0
  if (catalog.parentId) return catalog.id
  return catalog.children?.length ? 0 : catalog.id
}

export function getDisplayedCatalogCount(
  counts: CatalogServiceCounts | undefined,
  catalogId: number | null,
  kind?: "root" | "child",
) {
  if (!counts) return 0
  if (catalogId === null) return counts.total
  if (kind === "root") return counts.byRootCatalogId[catalogId] ?? 0
  return counts.byCatalogId[catalogId] ?? 0
}

export function shouldResetServiceForm({
  previousServiceId,
  nextServiceId,
  isDirty,
}: {
  previousServiceId: number | null
  nextServiceId: number
  isDirty: boolean
}) {
  return previousServiceId !== nextServiceId || !isDirty
}
