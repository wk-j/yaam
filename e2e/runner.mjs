import { spawn } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import { access, mkdir, mkdtemp, readFile, readdir, rename, rm, writeFile } from 'node:fs/promises'
import { constants } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { setTimeout as sleep } from 'node:timers/promises'

export const root = fileURLToPath(new URL('../', import.meta.url))
const archive = path.join(root, '.krypton/e2e/artifacts')
export const cases = [{
  id: 'web-query-happy-path',
  title: 'Add a Connection, run SELECT 1, and see the Execution Log',
  params: { connectionName: 'new' },
  fresh: { connectionName: 'new' },
  files: ['e2e/cases/web-query-happy-path.spec.ts', 'e2e/runner.mjs',
    'e2e/playwright.config.ts', 'e2e/package.json', 'e2e/package-lock.json'],
}]

class Blocked extends Error {}
export function parseParams(args) {
  const params = { connectionName: 'new' }
  for (let i = 0; i < args.length; i += 2) {
    if (args[i] !== '--param' || !args[i + 1]?.startsWith('connectionName=')) {
      throw new Error('Only --param connectionName=<name|new> is supported')
    }
    params.connectionName = args[i + 1].slice('connectionName='.length)
  }
  if (!/^[a-z0-9][a-z0-9_-]{0,62}$/.test(params.connectionName)) {
    throw new Error('connectionName must be 1–63 lowercase letters, digits, hyphens or underscores, starting with a letter or digit')
  }
  return params
}

// Never retain raw runner errors, headers or browser state. Redact child logs
// before writing them, including the app-generated token and private paths.
export function redact(text, privateValues = []) {
  let clean = String(text)
  for (const value of privateValues.filter(Boolean).sort((a, b) => b.length - a.length)) {
    clean = clean.split(value).join('[redacted]')
  }
  return clean.replace(/Bearer\s+[^\s"']+/gi, 'Bearer [redacted]')
    .replace(/(#token=)[^\s"'&]+/gi, '$1[redacted]')
    .replace(/(postgres(?:ql)?:\/\/)[^\s]+/gi, '$1[redacted]')
}

async function json(file, data) {
  await mkdir(path.dirname(file), { recursive: true })
  const tmp = `${file}.${randomUUID()}.tmp`
  await writeFile(tmp, `${JSON.stringify(data, null, 2)}\n`, { mode: 0o600 })
  await rename(tmp, file)
}

export async function history(directory = archive) {
  let entries
  try { entries = await readdir(directory, { withFileTypes: true }) }
  catch (error) { if (error.code === 'ENOENT') return []; throw error }
  const runs = []
  for (const entry of entries) {
    if (!entry.isDirectory()) continue
    try { runs.push(JSON.parse(await readFile(path.join(directory, entry.name, 'result.json'), 'utf8'))) }
    catch (error) {
      // An interrupted run may not have a terminal Result yet.
      if (error.code !== 'ENOENT') throw error
    }
  }
  return runs.sort((a, b) => b.startedAt.localeCompare(a.startedAt))
}

export function databaseConfig(env) {
  const names = ['YAAM_E2E_PGHOST', 'YAAM_E2E_PGPORT', 'YAAM_E2E_PGDATABASE', 'YAAM_E2E_PGUSER']
  const missing = names.filter(name => !env[name])
  if (env.YAAM_E2E_ALLOW_TEST_DB !== '1' || missing.length) {
    throw new Blocked(`Set YAAM_E2E_ALLOW_TEST_DB=1 and all of ${names.join(', ')} for a disposable passwordless test database`)
  }
  const port = Number(env.YAAM_E2E_PGPORT)
  if (!/^\d+$/.test(env.YAAM_E2E_PGPORT) || !Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Blocked('YAAM_E2E_PGPORT must be 1–65535')
  }
  // No DSNs or connection strings: credentials must never be accepted as a host.
  if (!/^[a-zA-Z0-9.:-]+$/.test(env.YAAM_E2E_PGHOST) ||
      !/^[a-zA-Z0-9_-]+$/.test(env.YAAM_E2E_PGDATABASE) ||
      !/^[a-zA-Z0-9_-]+$/.test(env.YAAM_E2E_PGUSER)) {
    throw new Blocked('Use a TCP hostname/IP and simple test database/role names, not a DSN or credentials')
  }
  return { host: env.YAAM_E2E_PGHOST, port, database: env.YAAM_E2E_PGDATABASE, user: env.YAAM_E2E_PGUSER }
}

async function run(caseId, args) {
  const runId = `${new Date().toISOString().replace(/[:.]/g, '-')}-${randomUUID().slice(0, 8)}`
  const directory = path.join(archive, runId)
  const resultFile = process.env.KRYPTON_E2E_RESULT
    ? path.resolve(process.env.KRYPTON_E2E_RESULT) : path.join(directory, 'result.json')
  const result = { version: 1, caseId, runId, verdict: 'ERROR', startedAt: new Date().toISOString(),
    finishedAt: '', env: 'isolated-local', versions: { node: process.version }, params: {}, checks: [], artifacts: [] }
  const children = new Set()
  const privateValues = []
  const logs = []
  let home, cancelled = false, stage = 'arguments'
  // Only safe, fixed messages reach Result. Child output is sanitized separately.
  const check = (label, status, detail, kind = 'precondition') => result.checks.push({ label, status, detail, kind })
  const signal = () => {
    cancelled = true
    for (const child of children) child.kill('SIGTERM')
  }
  process.on('SIGINT', signal)
  process.on('SIGTERM', signal)

  function start(command, argv, env = process.env) {
    if (cancelled) throw new Error('cancelled')
    const child = spawn(command, argv, { cwd: root, env, stdio: ['ignore', 'pipe', 'pipe'] })
    children.add(child)
    // Keep bounded output in memory until token/path redaction is available.
    let size = 0
    const capture = data => {
      if (size < 1024 * 1024) { logs.push(data.toString()); size += data.length }
    }
    child.stdout.on('data', capture)
    child.stderr.on('data', capture)
    child.done = new Promise(resolve => {
      child.once('error', () => { children.delete(child); resolve(-1) })
      child.once('close', code => { children.delete(child); resolve(code ?? -1) })
    })
    return child
  }
  async function bounded(child, ms) {
    const timer = setTimeout(() => child.kill('SIGTERM'), ms)
    const force = setTimeout(() => child.kill('SIGKILL'), ms + 5000)
    try { return await child.done } finally { clearTimeout(timer); clearTimeout(force) }
  }

  try {
    await mkdir(directory, { recursive: true })
    if (!cases.some(c => c.id === caseId)) throw new Error('unknown case')
    result.params = parseParams(args)
    if (result.params.connectionName === 'new') result.params.connectionName = `krypton-${randomUUID()}`
    stage = 'test database configuration'
    const database = databaseConfig(process.env)
    check(stage, 'pass', 'Explicit disposable database opt-in and target supplied')
    stage = 'macOS runtime'
    if (process.platform !== 'darwin') throw new Blocked('The application requires macOS and /usr/bin/security')
    try {
      await access('/usr/bin/security', constants.X_OK)
      await access(path.join(root, 'bin/yaam'), constants.X_OK)
      await access(path.join(root, 'internal/webui/dist/index.html'))
    } catch { throw new Blocked('Build the application with make build; macOS /usr/bin/security must be available') }
    stage = 'local test dependencies'
    let chromium, Client
    try {
      ;({ chromium } = await import('@playwright/test'))
      ;({ Client } = (await import('pg')).default)
    } catch { throw new Blocked('Install local test packages with npm ci --prefix e2e --ignore-scripts') }
    result.versions.playwright = JSON.parse(await readFile(new URL('./node_modules/@playwright/test/package.json', import.meta.url), 'utf8')).version
    stage = 'Chromium browser'
    let browser
    try { browser = await chromium.launch({ headless: true, timeout: 10000 }) }
    catch { throw new Blocked('Compatible Chromium unavailable; manually provision the browser with cd e2e && npx playwright install chromium') }
    await browser.close()
    check(stage, 'pass', 'Installed Chromium launches headlessly')
    stage = 'passwordless test database'
    const client = new Client({ ...database, password: () => '', ssl: false,
      connectionTimeoutMillis: 5000, query_timeout: 5000, application_name: 'yaam-krypton-e2e' })
    client.on('error', () => {})
    try { await client.connect(); await client.query('SELECT 1') }
    catch { throw new Blocked('Cannot query the supplied test database without a password; check the target and trust authentication') }
    finally { await client.end().catch(() => {}) }
    check(stage, 'pass', 'Test PostgreSQL accepts a passwordless SELECT 1')
    stage = 'Keychain isolation'
    const keyCheck = start('/usr/bin/security', ['find-generic-password', '-s', 'yaam', '-a', result.params.connectionName])
    const keyCode = await bounded(keyCheck, 5000)
    if (keyCode !== 44) throw new Blocked('Connection name already has a Keychain item, or Keychain is unavailable; use connectionName=new')
    check(stage, 'pass', 'No existing Keychain item for this Connection; no passwords will be written or deleted')
    stage = 'isolated app readiness'
    // /tmp is short enough for the macOS Unix socket path limit.
    home = await mkdtemp('/tmp/yaam-e2e-')
    privateValues.push(home)
    const env = { ...process.env, YAAM_HOME: home, PGPASSFILE: '/dev/null' }
    for (const key of Object.keys(env)) if (key.startsWith('PG') && key !== 'PGPASSFILE') delete env[key]
    const app = start(path.join(root, 'bin/yaam'), ['serve', '-addr', '127.0.0.1:0'], env)
    let info, ready = false
    const deadline = Date.now() + 15000
    while (Date.now() < deadline && children.has(app) && !cancelled) {
      try {
        info = JSON.parse(await readFile(path.join(home, 'serve.json'), 'utf8'))
        if (typeof info.token !== 'string' || !/^http:\/\/127\.0\.0\.1:\d+$/.test(info.url)) throw new Error('bad serve info')
        if (!privateValues.includes(info.token)) privateValues.push(info.token)
        const response = await fetch(`${info.url}/api/state`, {
          headers: { Authorization: `Bearer ${info.token}` }, signal: AbortSignal.timeout(1000),
        })
        if (response.ok) { ready = true; break }
      } catch { /* Startup may not have written serve.json or bound HTTP yet. */ }
      await sleep(100)
    }
    if (!ready) throw new Blocked('The isolated application did not become ready within 15 seconds; inspect the sanitized process log')
    check(stage, 'pass', 'Isolated loopback app responds to authenticated GET /api/state')
    await json(path.join(home, 'input.json'), { database, connectionName: result.params.connectionName, directory })
    stage = 'Playwright case'
    const pw = start(process.execPath, ['e2e/node_modules/@playwright/test/cli.js', 'test',
      '--config', 'e2e/playwright.config.ts', 'web-query-happy-path.spec.ts'],
    { ...env, YAAM_E2E_PRIVATE_HOME: home })
    const code = await bounded(pw, 90000)
    const outcome = JSON.parse(await readFile(path.join(home, 'outcome.json'), 'utf8'))
    if (!['PASS', 'FAIL', 'ERROR'].includes(outcome.verdict) || !Array.isArray(outcome.checks)) throw new Error('invalid outcome')
    result.checks.push(...outcome.checks)
    result.verdict = outcome.verdict
    if (result.verdict === 'PASS' && code !== 0) result.verdict = 'ERROR'
    for (const name of ['result.png', 'execution-log.png']) {
      try {
        await access(path.join(directory, name))
        result.artifacts.push({ label: name, path: path.relative(root, path.join(directory, name)) })
      } catch { /* A failing case might not have reached either screenshot. */ }
    }
    result.message = result.verdict === 'PASS' ? 'All web query expectations passed' : 'See individual checks and the sanitized runner log'
  } catch (error) {
    result.verdict = error instanceof Blocked && !cancelled ? 'BLOCKED' : 'ERROR'
    const detail = cancelled ? 'Run cancelled' : error instanceof Blocked || stage === 'arguments'
      ? (stage === 'arguments' && !cases.some(c => c.id === caseId) ? 'Unknown case id' : error.message)
      : `Harness error during ${stage}; see sanitized runner log`
    check(stage, result.verdict === 'BLOCKED' ? 'blocked' : 'info', detail,
      result.verdict === 'BLOCKED' ? 'precondition' : undefined)
    result.message = detail
  } finally {
    for (const child of [...children]) {
      child.kill('SIGTERM')
      await bounded(child, 5000)
    }
    // Last chance to obtain the generated token before sanitizing any output.
    if (home) {
      try { privateValues.push(JSON.parse(await readFile(path.join(home, 'serve.json'), 'utf8')).token) } catch {}
      await rm(home, { recursive: true, force: true })
    }
    process.off('SIGINT', signal)
    process.off('SIGTERM', signal)
    const logFile = path.join(directory, 'runner.log')
    await mkdir(directory, { recursive: true })
    await writeFile(logFile, redact(logs.join(''), privateValues) || `${result.message}\n`)
    result.artifacts.push({ label: 'runner log', path: path.relative(root, logFile) })
    const observations = path.join(directory, 'observations.json')
    await json(observations, { checks: result.checks })
    result.artifacts.push({ label: 'observations', path: path.relative(root, observations) })
    result.checks.forEach((item, index) => {
      item.evidence = { artifact: 'observations', pointer: `/checks/${index}` }
    })
    result.finishedAt = new Date().toISOString()
    await json(path.join(directory, 'result.json'), result)
    if (resultFile !== path.join(directory, 'result.json')) await json(resultFile, result)
  }
  console.log(`${result.verdict}: ${result.caseId} (${result.runId}) — ${result.message}`)
  return result.verdict === 'PASS' ? 0 : result.verdict === 'BLOCKED' ? 2 : 1
}

export async function main(argv) {
  const [command, caseId, ...args] = argv
  if (command === 'list' && !caseId) { console.log(JSON.stringify({ version: 1, cases })); return 0 }
  if (command === 'history' && !caseId) { console.log(JSON.stringify({ version: 1, runs: await history() })); return 0 }
  if (command === 'run' && caseId) return run(caseId, args)
  console.error('Usage: node e2e/runner.mjs list | history | run <caseId> [--param connectionName=<name|new>]')
  return 1
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main(process.argv.slice(2)).then(code => { process.exitCode = code }).catch(() => {
    console.error('Unable to read/write e2e evidence; check directory permissions')
    process.exitCode = 1
  })
}
