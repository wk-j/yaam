import { test, expect } from '@playwright/test'
import { readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'

type Check = {
  kind: 'step' | 'expect'
  step: string
  label: string
  status: 'pass' | 'fail' | 'info'
  detail: string
  target: { type: 'ui'; name: string }
  expected?: string
  actual?: string
}

const sql = 'SELECT 1 AS krypton_e2e_value'

test('web-query-happy-path', async ({ page }) => {
  const home = process.env.YAAM_E2E_PRIVATE_HOME
  if (!home) throw new Error('Run this case through node e2e/runner.mjs')
  const input = JSON.parse(await readFile(path.join(home, 'input.json'), 'utf8'))
  const info = JSON.parse(await readFile(path.join(home, 'serve.json'), 'utf8'))
  const checks: Check[] = []
  let verdict = 'ERROR'
  let assertionFailed = false
  let current = 'open-app'
  const step = async (id: string, label: string, action: () => Promise<unknown>) => {
    current = id
    const check: Check = { kind: 'step', step: id, label, status: 'pass', detail: 'Completed browser action',
      target: { type: 'ui', name: label } }
    checks.push(check)
    try { await action() }
    catch {
      check.status = 'fail'
      check.detail = 'Browser action could not complete'
      throw new Error(`Browser action did not complete: ${label}`)
    }
  }
  const verify = async (label: string, expected: string, assertion: () => Promise<unknown>) => {
    const check: Check = { kind: 'expect', step: current, label, status: 'pass', detail: expected,
      expected, actual: expected, target: { type: 'ui', name: label } }
    try { await assertion() }
    catch {
      assertionFailed = true
      check.status = 'fail'
      check.actual = 'Expectation not met within the timeout'
      check.detail = check.actual
      // Keep secrets, complete page DOM, and arbitrary database error text out
      // of observations and the Playwright error report.
      throw new Error(`Expectation not met: ${label}`)
    } finally { checks.push(check) }
  }
  try {
    // Authenticate before app startup, without a token-bearing navigation URL.
    // Traces/storageState/HAR are deliberately disabled.
    await page.addInitScript(({ token, origin }) => {
      if (location.origin === origin) sessionStorage.setItem('yaam.token', token)
    }, { token: info.token, origin: info.url })
    await step('open-app', 'Open isolated application', () => page.goto(info.url))
    await verify('Connections navigation is usable', 'Connections button is visible',
      () => expect(page.getByTestId('nav-connections')).toBeVisible())
    await step('add-connection', 'Add test Connection', async () => {
      await page.getByTestId('nav-connections').click()
      await page.getByTestId('connection-add').click()
      await page.getByLabel('Name', { exact: true }).fill(input.connectionName)
      await page.getByLabel('Host', { exact: true }).fill(input.database.host)
      await page.getByLabel('Port', { exact: true }).fill(String(input.database.port))
      await page.getByLabel('Database', { exact: true }).fill(input.database.database)
      await page.getByLabel('Role', { exact: true }).fill(input.database.user)
      // Leave Password empty: no Keychain write or deletion is requested.
      await page.getByTestId('connection-save').click()
    })
    const connection = page.getByTestId('connection-row').filter({ has: page.getByText(input.connectionName, { exact: true }) })
    await verify('Connection is listed', 'One test Connection is visible', () => expect(connection).toHaveCount(1))
    await step('test-connection', 'Test Connection', () => connection.getByTestId('connection-test').click())
    await verify('Connection test succeeds', 'Test passed: connected and ran SELECT 1.',
      () => expect(connection.getByTestId('connection-test-result')).toHaveText('Test passed: connected and ran SELECT 1.'))
    await step('run-query', 'Run read-only SQL in the editor', async () => {
      await page.getByTestId('sidebar-connection').filter({ hasText: input.connectionName }).click()
      await page.getByTestId('editor-connection').selectOption(input.connectionName)
      await page.getByTestId('sql-editor-input').fill(sql)
      await page.getByTestId('editor-run').click()
    })
    const results = page.getByTestId('editor-results')
    await verify('Result column', 'krypton_e2e_value',
      () => expect(results.getByTestId('result-column-name')).toHaveText(['krypton_e2e_value']))
    await verify('Result value', '1', () => expect(results.getByTestId('result-cell')).toHaveText(['1']))
    await verify('No SQL error', 'No SQL error is displayed', () => expect(results.getByTestId('editor-sql-error')).toHaveCount(0))
    await results.screenshot({ path: path.join(input.directory, 'result.png') })
    await step('execution-log', 'Open Execution Log', () => page.getByTestId('nav-execution-log').click())
    const log = page.getByTestId('execution-log')
    const entry = log.getByRole('row').filter({ has: page.getByTestId('execution-log-sql').filter({ hasText: sql }) })
    await verify('SQL is logged once', sql, () => expect(entry).toHaveCount(1))
    await verify('Logged Connection', input.connectionName,
      () => expect(entry.getByTestId('execution-log-connection')).toHaveText(input.connectionName))
    await verify('Logged execution succeeded', 'ok', () => expect(entry.getByTestId('execution-log-status')).toHaveText('ok'))
    await log.screenshot({ path: path.join(input.directory, 'execution-log.png') })
    verdict = 'PASS'
  } catch {
    verdict = assertionFailed ? 'FAIL' : 'ERROR'
    // Fixed message only: do not let Playwright retain arbitrary page values.
    throw new Error(`${verdict}: web query did not complete; see normalized checks`)
  } finally {
    await writeFile(path.join(home, 'outcome.json'), JSON.stringify({ verdict, checks }), { mode: 0o600 })
  }
})
