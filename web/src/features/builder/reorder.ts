/**
 * Computes the reordered list AND the anchor the API needs, from one place, so
 * the optimistic update and the request can never disagree.
 *
 * The API takes `before` or `after` naming a SIBLING, never a position, so the
 * anchor is read off the list after the move.
 */
export function moveWithin<T extends { id: string }>(
  items: T[],
  id: string,
  direction: -1 | 1,
): { next: T[]; before?: string; after?: string } {
  const from = items.findIndex((i) => i.id === id)
  if (from < 0) return { next: items }

  const to = from + direction
  if (to < 0 || to >= items.length) return { next: items }

  const next = items.slice()
  const [moved] = next.splice(from, 1)
  next.splice(to, 0, moved)

  const position = next.findIndex((i) => i.id === id)
  if (direction === -1) {
    // Moving up: sit before whatever now follows.
    const successor = next[position + 1]
    return { next, before: successor?.id }
  }
  // Moving down: sit after whatever now precedes.
  const predecessor = next[position - 1]
  return { next, after: predecessor?.id }
}
