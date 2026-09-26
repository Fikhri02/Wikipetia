import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Field } from './Field'

describe('Field', () => {
  it('gives every instance a unique id, so two same-labelled fields do not collide', () => {
    // The builder renders one "Block title" field PER MODULE, and the spec's
    // definition of done requires a course with two modules. With ids derived
    // from the label, both inputs share an id and both labels point at the
    // first one — clicking the second module's label focuses the first module's
    // input, and getByLabel resolves to two elements.
    render(
      <>
        <Field label="Block title" defaultValue="first" />
        <Field label="Block title" defaultValue="second" />
      </>,
    )

    const inputs = screen.getAllByLabelText('Block title')
    expect(inputs).toHaveLength(2)

    const ids = inputs.map((i) => i.getAttribute('id'))
    expect(ids[0]).toBeTruthy()
    expect(ids[0]).not.toBe(ids[1])

    // And each label must actually point at its own input.
    expect((inputs[0] as HTMLInputElement).value).toBe('first')
    expect((inputs[1] as HTMLInputElement).value).toBe('second')
  })

  it('still honours an explicit id', () => {
    render(<Field label="Course title" id="explicit-id" />)
    expect(screen.getByLabelText('Course title')).toHaveAttribute('id', 'explicit-id')
  })

  it('ties its error message to the input for assistive tech', () => {
    render(<Field label="Title" error="must not be empty" />)
    const input = screen.getByLabelText('Title')
    expect(input).toHaveAttribute('aria-invalid', 'true')
    const errorId = input.getAttribute('aria-errormessage')
    expect(errorId).toBeTruthy()
    expect(document.getElementById(errorId!)).toHaveTextContent('must not be empty')
  })
})
