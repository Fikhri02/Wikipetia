import { useState } from 'react'
import { devUser, setDevUser } from '../api/client'

export function DevUserSwitcher() {
  const [name, setName] = useState(devUser)

  return (
    <form
      className="flex items-center gap-2 text-xs text-muted"
      onSubmit={(e) => {
        e.preventDefault()
        setDevUser(name)
        // Every cached query belongs to the previous user.
        window.location.reload()
      }}
    >
      <label htmlFor="dev-user">Signed in as</label>
      <input
        id="dev-user"
        value={name}
        onChange={(e) => setName(e.target.value)}
        className="w-24 rounded border border-line px-1.5 py-0.5 text-ink"
      />
      <button type="submit" className="rounded border border-line px-1.5 py-0.5 hover:bg-slate-50">
        switch
      </button>
    </form>
  )
}
