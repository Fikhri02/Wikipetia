import { test, expect } from '@playwright/test'

/**
 * The slice 1 definition of done, in a browser, from an empty course:
 * build a course, enrol, complete a reading, take a quiz, see progress and
 * mastery move. The video block is exercised by the API tests rather than here
 * — driving a real YouTube player in CI is slow and flaky, and its completion
 * path is covered by TestVideoCompletesOnPlayerEndEvent on the server.
 */
test('build a course, study it, and see mastery move', async ({ page }) => {
  const unique = Date.now()
  const courseTitle = `E2E Chemistry ${unique}`

  await page.goto('/build')
  await expect(page.getByRole('heading', { name: 'Build' })).toBeVisible({ timeout: 10_000 })

  // --- build ---
  await page.getByLabel('Course title').fill(courseTitle)
  await page.getByRole('button', { name: 'Create course' }).click()
  await page.getByText(courseTitle).click()

  // The definition of done specifies FOUR topics and TWO modules; run that,
  // not a reduced version of it.
  for (const topic of ['Atoms', 'Bonding', 'Moles', 'Acids']) {
    await page.getByLabel('New topic').fill(topic)
    // `exact` matters: "Add" is a substring of "Add module", "Add block",
    // "Add option" and "Add question", and Playwright name matching is
    // substring-based by default.
    await page.getByRole('button', { name: 'Add', exact: true }).click()
  }
  await expect(page.getByText('4 topics')).toBeVisible()

  for (const mod of ['Module 1', 'Module 2']) {
    await page.getByLabel('New module').fill(mod)
    await page.getByRole('button', { name: 'Add module' }).click()
  }
  await expect(page.getByText('2 modules · 4 topics')).toBeVisible()

  // Reordering is the one optimistic write in the builder — exercise it.
  await page.getByRole('button', { name: 'Move Module 2 up' }).click()
  await expect(page.getByLabel('Module title: Module 2')).toBeVisible()
  await page.getByRole('button', { name: 'Move Module 2 down' }).click()

  // Two modules means two "Block title" fields, so scope to the first module's
  // list item. This is why Field uses useId rather than a label-derived id.
  const firstModule = page.getByRole('listitem').filter({ has: page.getByLabel('Module title: Module 1') })

  // A reading block.
  await firstModule.getByLabel('Block title').fill('What is an atom?')
  await firstModule.getByLabel('Block type').selectOption('reading')
  await firstModule.getByRole('button', { name: 'Add block' }).click()
  await firstModule.getByRole('button', { name: 'Edit What is an atom?' }).click()
  await firstModule.getByLabel('Body (Markdown)').fill('# Atoms\n\nAn atom is the smallest unit of an element.')
  await firstModule.getByRole('button', { name: 'Save reading' }).click()
  await firstModule.getByRole('button', { name: 'Atoms', exact: true }).click() // tag the topic

  // A quiz block with one question.
  await firstModule.getByLabel('Block title').fill('Check yourself')
  await firstModule.getByLabel('Block type').selectOption('quiz')
  await firstModule.getByRole('button', { name: 'Add block' }).click()
  await firstModule.getByRole('button', { name: 'Edit Check yourself' }).click()
  await firstModule.getByLabel('Kind').selectOption('true_false')
  await firstModule.getByLabel('Question').fill('Iron is an element')
  // exact: "Topic" is a substring of "New topic" in the topic panel, and
  // Playwright label matching is substring-based by default.
  await firstModule.getByLabel('Topic', { exact: true }).selectOption({ label: 'Atoms' })
  await firstModule.getByRole('button', { name: 'Add question' }).click()

  // --- study ---
  await page.getByRole('link', { name: 'Library' }).click()
  await page.getByRole('button', { name: `Start ${courseTitle}` }).click()
  // Enrolling moves the course out of "not yet started" and into the enrolment
  // list. Waiting for the button to go means the enrolment landed — and avoids
  // matching both the card title and the "Start <title>" button at once.
  await expect(page.getByRole('button', { name: `Start ${courseTitle}` })).toHaveCount(0)
  await page.getByText(courseTitle, { exact: true }).click()

  await expect(page.getByRole('progressbar', { name: 'Completed' })).toHaveAttribute(
    'aria-valuenow',
    '0',
  )

  await page.getByRole('button', { name: 'Mark as done' }).click()
  // Scoped to the reading's own article: "Completed" is also the label of the
  // progress bar, which is on screen throughout.
  await expect(
    page.getByRole('article').filter({ hasText: 'What is an atom?' }).getByText('Completed'),
  ).toBeVisible()

  await page.getByRole('button', { name: /Check yourself/ }).click()
  await page.getByRole('radio', { name: 'True' }).check()
  await page.getByRole('button', { name: 'Submit answers' }).click()

  // --- the payoff ---
  await expect(page.getByText('You scored 100%')).toBeVisible()
  await expect(page.getByRole('progressbar', { name: 'Completed' })).toHaveAttribute(
    'aria-valuenow',
    '100',
  )
  // One of four topics has evidence, so mastery is 25% — the completed/mastered
  // gap the spec exists to make visible.
  await expect(page.getByRole('progressbar', { name: 'Mastered' })).toHaveAttribute(
    'aria-valuenow',
    '25',
  )
})
