export interface TreeNode {
  id: number
  name: string
  code: string
  parentId: number | null
  managerId: number | null
  managerName: string
  sort: number
  description: string
  isActive: boolean
  memberCount: number
  createdAt: string
  updatedAt: string
  children?: TreeNode[]
}

export interface MemberPositionItem {
  assignmentId: number
  positionId: number
  positionName: string
  isPrimary: boolean
}

export interface MemberWithPositions {
  userId: number
  username: string
  email: string
  avatar: string
  departmentId: number
  positions: MemberPositionItem[]
  createdAt: string
}

export interface PositionItem {
  id: number
  name: string
  code: string
  isActive: boolean
}

export interface UserItem {
  id: number
  username: string
  email: string
  avatar: string
}

export function collectAllIds(nodes: TreeNode[]): number[] {
  const ids: number[] = []
  for (const n of nodes) {
    ids.push(n.id)
    if (n.children) ids.push(...collectAllIds(n.children))
  }
  return ids
}

export function collectExpandedIds(nodes: TreeNode[], maxDepth: number, depth = 0): number[] {
  const ids: number[] = []
  for (const node of nodes) {
    if (depth < maxDepth && node.children?.length) {
      ids.push(node.id)
      ids.push(...collectExpandedIds(node.children, maxDepth, depth + 1))
    }
  }
  return ids
}

export function findNodeById(nodes: TreeNode[], id: number): TreeNode | null {
  for (const node of nodes) {
    if (node.id === id) return node
    if (node.children) {
      const result = findNodeById(node.children, id)
      if (result) return result
    }
  }
  return null
}

export function filterTree(nodes: TreeNode[], keyword: string): TreeNode[] {
  if (!keyword) return nodes
  const lower = keyword.toLowerCase()
  const result: TreeNode[] = []
  for (const node of nodes) {
    const childMatches = node.children ? filterTree(node.children, keyword) : []
    const selfMatches =
      node.name.toLowerCase().includes(lower) || node.code.toLowerCase().includes(lower)
    if (selfMatches || childMatches.length > 0) {
      result.push({ ...node, children: selfMatches ? node.children : childMatches })
    }
  }
  return result
}
