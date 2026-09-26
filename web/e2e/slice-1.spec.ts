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

  await page.getByLabel('New topic').fill('Atoms')
  // `exact` matters: "Add" is a substring of "Add module", "Add block",
  // "Add option" and "Add question", and Playwright name matching is
  // substring-based by default.
  await page.getByRole('button', { name: 'Add', exact: true }).click()

  await page.getByLabel('New module').fill('Module 1')
  await page.getByRole('button', { name: 'Add module' }).click()

  // A reading block.
  await page.getByLabel('Block title').fill('What is an atom?')
  await page.getByLabel('Block type').selectOption('reading')
  await page.getByRole('button', { name: 'Add block' }).click()
  await page.getByRole('button', { name: 'Edit What is an atom?' }).click()
  await page.getByLabel('Body (Markdown)').fill('# Atoms\n\nAn atom is the smallest unit of an element.')
  await page.getByRole('button', { name: 'Save reading' }).click()
  await page.getByRole('button', { name: 'Atoms', exact: true }).click() // tag the topic

  // A quiz block with one question.
  await page.getByLabel('Block title').fill('Check yourself')
  await page.getByLabel('Block type').selectOption('quiz')
  await page.getByRole('button', { name: 'Add block' }).click()
  await page.getByRole('button', { name: 'Edit Check yourself' }).click()
  await page.getByLabel('Kind').selectOption('true_false')
  await page.getByLabel('Question').fill('Iron is an element')
  // exact: "Topic" is a substring of "New topic" in the topic panel, and
  // Playwright label matching is substring-based by default.
  await page.getByLabel('Topic', { exact: true }).selectOption({ label: 'Atoms' })
  await page.getByRole('button', { name: 'Add question' }).click()

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
  await expect(page.getByText('Completed', { exact: true })).toBeVisible()

  await page.getByRole('button', { name: /Check yourself/ }).click()
  await page.getByRole('radio', { name: 'True' }).check()
  await page.getByRole('button', { name: 'Submit answers' }).click()

  // --- the payoff ---
  await expect(page.getByText('You scored 100%')).toBeVisible()
  await expect(page.getByRole('progressbar', { name: 'Completed' })).toHaveAttribute(
    'aria-valuenow',
    '100',
  )
  await expect(page.getByRole('progressbar', { name: 'Mastered' })).toHaveAttribute(
    'aria-valuenow',
    '100',
  )
})
