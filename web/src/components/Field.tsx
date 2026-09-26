import type { InputHTMLAttributes } from 'react'

type Props = InputHTMLAttributes<HTMLInputElement> & {
  label: string
  error?: string
}

export function Field({ label, error, id, className = '', ...rest }: Props) {
  const inputId = id ?? `field-${label.toLowerCase().replace(/\s+/g, '-')}`
  return (
    <div className="space-y-1">
      <label htmlFor={inputId} className="block text-sm font-medium">
        {label}
      </label>
      <input
        id={inputId}
        aria-invalid={error ? true : undefined}
        aria-errormessage={error ? `${inputId}-error` : undefined}
        {...rest}
        className={`w-full rounded-md border px-2.5 py-1.5 text-sm outline-none focus:ring-2 focus:ring-accent/30 ${
          error ? 'border-red-400' : 'border-line'
        } ${className}`}
      />
      {error && (
        <p id={`${inputId}-error`} className="text-xs text-red-700">
          {error}
        </p>
      )}
    </div>
  )
}
