export type ConnectionStatus = 'ok' | 'error' | 'unknown'
export type SslMode = 'disable' | 'require' | 'verify-ca' | 'verify-full'

export interface Connection {
  name: string
  host: string
  port: number
  database: string
  role: string
  sslmode: SslMode
  agentTimeoutSeconds: number
  hasPassword: boolean
  status: ConnectionStatus
  statusError: string
}

export interface Settings {
  agentLogLimit: number
  executionLogLimit: number
}

export interface Agent {
  id: string
  agentName: string
  project: string
  connection: string
  since: string
}

export type StatementKind = 'read' | 'write' | 'txcontrol'

export interface Pending {
  id: string
  time: string
  agentName: string
  project: string
  connection: string
  purpose: string
  sql: string
  statements: { text: string; kind: StatementKind }[]
}

export interface AppState {
  connections: Connection[]
  settings: Settings
  agents: Agent[]
  pending: Pending[]
}

export type AgentLogStatus = 'run' | 'waiting' | 'rejected' | 'cancelled' | 'failed'

export interface AgentLogEntry {
  id: number
  time: string
  agentName: string
  project: string
  connection: string
  kind: 'request' | 'schema'
  purpose: string
  sql: string
  status: AgentLogStatus
  durationMs: number | null
  rows: number | null
  error: string
}

export interface ExecutionLogEntry {
  id: number
  time: string
  by: 'user' | 'agent'
  agentName: string
  project: string
  connection: string
  source: 'editor' | 'grid' | 'agent'
  sql: string
  durationMs: number
  rows: number | null
  error: string
}

export interface SchemaColumn {
  name: string
  type: string
  pk: boolean
}

export interface SchemaTable {
  schema: string
  name: string
  kind: 'table' | 'view' | 'materialized view'
  columns: SchemaColumn[]
  hasPrimaryKey: boolean
}

export interface Schema {
  tables: SchemaTable[]
}

export interface ResultColumn {
  name: string
  type: string
}

export interface StatementResult {
  command: string
  columns: ResultColumn[]
  rows: (string | null)[][]
  rowCount: number
}

export interface Editable {
  schema: string
  table: string
  keyColumns: string[]
  columnSources: (string | null)[]
}

export interface RunResult {
  statements: StatementResult[]
  durationMs: number
  error: string
  notices: string[]
  editable: Editable | null
}

export type EditChange =
  | { op: 'update'; key: Record<string, string | null>; set: Record<string, string | null> }
  | { op: 'delete'; key: Record<string, string | null> }

export interface EditsResponse {
  sql: string[]
  durationMs?: number
  error?: string
}
