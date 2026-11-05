import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import { copyFile, mkdir, mkdtemp, readFile, readdir, realpath, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { cases, databaseConfig, history, parseParams, redact, root } from './runner.mjs'

// The runner locates its repo/archive relative to its own file. Run negative
// protocol tests from a disposable fixture so they cannot alter real history.
async function isolatedRunner(directory) {
  const runner = path.join(directory, 'e2e/runner.mjs')
  await mkdir(path.dirname(runner), { recursive: true })
  await copyFile(path.join(root, 'e2e/runner.mjs'), runner)
  return realpath(runner)
}

test('case list and parameters match the first case', () => {
  assert.equal(cases[0].id, 'web-query-happy-path')
  assert.deepEqual(parseParams([]), { connectionName: 'new' })
  assert.deepEqual(parseParams(['--param', 'connectionName=krypton-test']), { connectionName: 'krypton-test' })
  for (const args of [['--param'], ['--param', 'password=not-logged'], ['--param', 'connectionName=../bad'], ['--fresh']]) {
    assert.throws(() => parseParams(args))
  }
})

test('database requires explicit opt-in and all target inputs; never accepts credentials as a host', () => {
  const env = { YAAM_E2E_ALLOW_TEST_DB: '1', YAAM_E2E_PGHOST: '127.0.0.1', YAAM_E2E_PGPORT: '5432',
    YAAM_E2E_PGDATABASE: 'yaam_test', YAAM_E2E_PGUSER: 'yaam_test' }
  assert.equal(databaseConfig(env).port, 5432)
  assert.throws(() => databaseConfig({}))
  assert.throws(() => databaseConfig({ ...env, YAAM_E2E_ALLOW_TEST_DB: '' }))
  assert.throws(() => databaseConfig({ ...env, YAAM_E2E_PGPORT: '0' }))
  assert.throws(() => databaseConfig({ ...env, YAAM_E2E_PGHOST: 'postgres://user:secret@host/db' }))
})

test('logs redact token, bearer, token URLs, DSNs and private paths', () => {
  const text = 'sensitive-token /tmp/private-home Bearer abc #token=abc postgresql://user:secret@host/db'
  const clean = redact(text, ['sensitive-token', '/tmp/private-home'])
  for (const value of ['sensitive-token', '/tmp/private-home', 'abc', 'secret']) assert.ok(!clean.includes(value))
})

test('history is read-only and preserves run identities', async () => {
  const dir = await mkdtemp(path.join(tmpdir(), 'yaam-history-test-'))
  try {
    const empty = path.join(dir, 'absent')
    assert.deepEqual(await history(empty), [])
    assert.deepEqual(await readdir(dir), [])
    await mkdir(path.join(dir, 'one'))
    const result = { version: 1, caseId: cases[0].id, runId: 'stable-id', startedAt: '2026-11-05T00:00:00Z' }
    await writeFile(path.join(dir, 'one/result.json'), JSON.stringify(result))
    assert.deepEqual(await history(dir), [result])
    assert.equal((await history(dir))[0].runId, 'stable-id')
  } finally { await rm(dir, { recursive: true, force: true }) }
})

test('list works without database inputs and emits only JSON', () => {
  const listed = JSON.parse(execFileSync(process.execPath, ['e2e/runner.mjs', 'list'], { cwd: root, encoding: 'utf8' }))
  assert.deepEqual(listed, { version: 1, cases })
})

test('missing database inputs write BLOCKED to nested Result path, with retained evidence and stable history', async () => {
  const dir = await mkdtemp(path.join(tmpdir(), 'yaam-result-test-'))
  const env = { ...process.env, KRYPTON_E2E_RESULT: path.join(dir, 'nested/result.json') }
  for (const name of Object.keys(env)) if (name.startsWith('YAAM_E2E_')) delete env[name]
  const realHistory = await history()
  try {
    const runner = await isolatedRunner(dir)
    const run = spawnSync(process.execPath, [runner, 'run', cases[0].id], { cwd: dir, env, encoding: 'utf8', timeout: 10000 })
    assert.equal(run.status, 2, run.stderr)
    const result = JSON.parse(await readFile(env.KRYPTON_E2E_RESULT, 'utf8'))
    assert.equal(result.verdict, 'BLOCKED')
    assert.equal(result.checks[0].kind, 'precondition')
    assert.equal(result.checks[0].status, 'blocked')
    assert.equal((await history(path.join(dir, '.krypton/e2e/artifacts'))).find(r => r.runId === result.runId).caseId, cases[0].id)
    assert.deepEqual(await history(), realHistory, 'Negative tests must not alter real e2e history')
    for (const artifact of result.artifacts) {
      assert.ok(!path.isAbsolute(artifact.path))
      assert.ok(!artifact.path.split('/').includes('..'))
      await readFile(path.join(dir, artifact.path))
    }
  } finally { await rm(dir, { recursive: true, force: true }) }
})

test('invalid parameters write ERROR without echoing user input', async () => {
  const dir = await mkdtemp(path.join(tmpdir(), 'yaam-error-test-'))
  const env = { ...process.env, KRYPTON_E2E_RESULT: path.join(dir, 'result.json') }
  const realHistory = await history()
  try {
    const runner = await isolatedRunner(dir)
    const run = spawnSync(process.execPath, [runner, 'run', cases[0].id, '--param', 'password=private-value'],
      { cwd: dir, env, encoding: 'utf8', timeout: 10000 })
    assert.equal(run.status, 1, run.stderr)
    const raw = await readFile(env.KRYPTON_E2E_RESULT, 'utf8')
    assert.equal(JSON.parse(raw).verdict, 'ERROR')
    assert.ok(!raw.includes('private-value'))
    assert.ok(!run.stdout.includes('private-value'))
    assert.equal((await history(path.join(dir, '.krypton/e2e/artifacts'))).length, 1)
    assert.deepEqual(await history(), realHistory, 'Negative tests must not alter real e2e history')
  } finally { await rm(dir, { recursive: true, force: true }) }
})
