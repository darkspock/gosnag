import { Button } from './button'
import { Input } from './input'
import { Select } from './select'
import { Plus, Trash2, GitBranch } from 'lucide-react'
import { cn } from '@/lib/utils'

// --- Types ---

export interface ConditionNode {
  // Leaf condition
  type?: string
  op?: string
  value?: unknown

  // Group
  operator?: 'and' | 'or'
  conditions?: ConditionNode[]
}

export interface ConditionGroup {
  operator: 'and' | 'or'
  conditions: ConditionNode[]
}

interface ConditionTypeInfo {
  label: string
  category: 'string' | 'number' | 'text'
  operators: { value: string; label: string }[]
}

const CONDITION_TYPES: Record<string, ConditionTypeInfo> = {
  level: {
    label: 'Level',
    category: 'string',
    operators: [
      { value: 'in', label: 'is one of' },
      { value: 'not_in', label: 'is not one of' },
      { value: 'eq', label: 'equals' },
      { value: 'neq', label: 'not equals' },
    ],
  },
  platform: {
    label: 'Platform',
    category: 'string',
    operators: [
      { value: 'eq', label: 'equals' },
      { value: 'neq', label: 'not equals' },
      { value: 'in', label: 'is one of' },
    ],
  },
  title: {
    label: 'Title',
    category: 'text',
    operators: [
      { value: 'contains', label: 'contains' },
      { value: 'not_contains', label: 'does not contain' },
      { value: 'matches', label: 'matches regex' },
    ],
  },
  event_data: {
    label: 'Event data (full)',
    category: 'text',
    operators: [
      { value: 'contains', label: 'contains' },
      { value: 'not_contains', label: 'does not contain' },
      { value: 'matches', label: 'matches regex' },
    ],
  },
  total_events: {
    label: 'Total events',
    category: 'number',
    operators: [
      { value: 'gte', label: '>=' },
      { value: 'gt', label: '>' },
      { value: 'lte', label: '<=' },
      { value: 'lt', label: '<' },
      { value: 'eq', label: '=' },
    ],
  },
  velocity_1h: {
    label: 'Events in last hour',
    category: 'number',
    operators: [
      { value: 'gte', label: '>=' },
      { value: 'gt', label: '>' },
      { value: 'lte', label: '<=' },
      { value: 'lt', label: '<' },
    ],
  },
  velocity_24h: {
    label: 'Events in last 24h',
    category: 'number',
    operators: [
      { value: 'gte', label: '>=' },
      { value: 'gt', label: '>' },
      { value: 'lte', label: '<=' },
      { value: 'lt', label: '<' },
    ],
  },
  user_count: {
    label: 'Affected users',
    category: 'number',
    operators: [
      { value: 'gte', label: '>=' },
      { value: 'gt', label: '>' },
      { value: 'lte', label: '<=' },
      { value: 'lt', label: '<' },
    ],
  },
  environment: {
    label: 'Environment',
    category: 'string',
    operators: [
      { value: 'eq', label: 'equals' },
      { value: 'neq', label: 'not equals' },
    ],
  },
  release: {
    label: 'Release',
    category: 'string',
    operators: [
      { value: 'eq', label: 'equals' },
      { value: 'neq', label: 'not equals' },
      { value: 'contains', label: 'contains' },
    ],
  },
  priority: {
    label: 'Priority score',
    category: 'number',
    operators: [
      { value: 'gte', label: '>=' },
      { value: 'gt', label: '>' },
      { value: 'lte', label: '<=' },
      { value: 'lt', label: '<' },
      { value: 'eq', label: '=' },
    ],
  },
}

// --- Component ---

interface ConditionBuilderProps {
  value: ConditionGroup | null
  onChange: (value: ConditionGroup) => void
  availableTypes?: string[]
}

export function ConditionBuilder({ value, onChange, availableTypes }: ConditionBuilderProps) {
  const group = value || { operator: 'and', conditions: [] }
  const types = availableTypes
    ? Object.entries(CONDITION_TYPES).filter(([k]) => availableTypes.includes(k))
    : Object.entries(CONDITION_TYPES)

  const updateGroup = (g: ConditionGroup) => onChange(g)

  const addCondition = () => {
    const firstType = types[0]?.[0] || 'level'
    const firstOp = CONDITION_TYPES[firstType]?.operators[0]?.value || 'eq'
    updateGroup({
      ...group,
      conditions: [...group.conditions, { type: firstType, op: firstOp, value: '' }],
    })
  }

  const addSubGroup = () => {
    updateGroup({
      ...group,
      conditions: [...group.conditions, { operator: 'or', conditions: [] }],
    })
  }

  const removeCondition = (index: number) => {
    updateGroup({
      ...group,
      conditions: group.conditions.filter((_, i) => i !== index),
    })
  }

  const updateCondition = (index: number, node: ConditionNode) => {
    const next = [...group.conditions]
    next[index] = node
    updateGroup({ ...group, conditions: next })
  }

  const toggleOperator = () => {
    updateGroup({ ...group, operator: group.operator === 'and' ? 'or' : 'and' })
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2 mb-2">
        <button
          type="button"
          onClick={toggleOperator}
          className={cn(
            'text-xs font-semibold px-2 py-1 rounded border transition-colors',
            group.operator === 'and'
              ? 'bg-blue-500/15 text-blue-400 border-blue-500/30'
              : 'bg-amber-500/15 text-amber-400 border-amber-500/30'
          )}
        >
          {group.operator === 'and' ? 'ALL match (AND)' : 'ANY match (OR)'}
        </button>
        <span className="text-xs text-muted-foreground">
          {group.conditions.length === 0 ? 'No conditions — always matches' : ''}
        </span>
      </div>

      {group.conditions.map((node, i) => (
        <div key={i}>
          {i > 0 && (
            <div className="flex items-center gap-2 my-1 ml-4">
              <span className="text-[10px] font-mono text-muted-foreground/60 uppercase">
                {group.operator}
              </span>
              <div className="flex-1 border-t border-border/30" />
            </div>
          )}
          {node.operator ? (
            <div className="ml-4 pl-3 border-l-2 border-border/40">
              <ConditionBuilder
                value={node as ConditionGroup}
                onChange={(sub) => updateCondition(i, sub)}
                availableTypes={availableTypes}
              />
              <button
                type="button"
                onClick={() => removeCondition(i)}
                className="text-xs text-muted-foreground hover:text-destructive mt-1"
              >
                Remove group
              </button>
            </div>
          ) : (
            <LeafCondition
              node={node}
              types={types}
              onChange={(n) => updateCondition(i, n)}
              onRemove={() => removeCondition(i)}
            />
          )}
        </div>
      ))}

      <div className="flex gap-2 mt-2">
        <Button type="button" variant="outline" size="sm" onClick={addCondition}>
          <Plus className="h-3 w-3 mr-1" /> Condition
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={addSubGroup}>
          <GitBranch className="h-3 w-3 mr-1" /> Group
        </Button>
      </div>
    </div>
  )
}

// --- Leaf Condition Row ---

function LeafCondition({
  node,
  types,
  onChange,
  onRemove,
}: {
  node: ConditionNode
  types: [string, ConditionTypeInfo][]
  onChange: (n: ConditionNode) => void
  onRemove: () => void
}) {
  const typeInfo = CONDITION_TYPES[node.type || '']
  const operators = typeInfo?.operators || []

  const handleTypeChange = (newType: string) => {
    const info = CONDITION_TYPES[newType]
    const firstOp = info?.operators[0]?.value || 'eq'
    onChange({ type: newType, op: firstOp, value: '' })
  }

  const handleOpChange = (newOp: string) => {
    onChange({ ...node, op: newOp })
  }

  const handleValueChange = (val: string) => {
    if (typeInfo?.category === 'number') {
      onChange({ ...node, value: val === '' ? '' : Number(val) })
    } else if (node.op === 'in' || node.op === 'not_in') {
      onChange({ ...node, value: val.split(',').map(s => s.trim()).filter(Boolean) })
    } else {
      onChange({ ...node, value: val })
    }
  }

  const displayValue = (): string => {
    if (Array.isArray(node.value)) return (node.value as string[]).join(', ')
    if (node.value === undefined || node.value === null) return ''
    return String(node.value)
  }

  return (
    <div className="ml-4 rounded-md border border-border/40 bg-muted/20 p-2 space-y-2">
      <div className="flex items-center gap-2">
        <Select
          value={node.type || ''}
          onChange={(e) => handleTypeChange(e.target.value)}
          className="flex-1 text-xs"
        >
          {types.map(([key, info]) => (
            <option key={key} value={key}>{info.label}</option>
          ))}
        </Select>

        <Select
          value={node.op || ''}
          onChange={(e) => handleOpChange(e.target.value)}
          className="flex-1 text-xs"
        >
          {operators.map(op => (
            <option key={op.value} value={op.value}>{op.label}</option>
          ))}
        </Select>

        <button
          type="button"
          onClick={onRemove}
          className="p-1 text-muted-foreground hover:text-destructive transition-colors shrink-0"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      </div>
      <Input
        value={displayValue()}
        onChange={(e) => handleValueChange(e.target.value)}
        placeholder={
          typeInfo?.category === 'number' ? 'e.g. 10'
          : (node.op === 'in' || node.op === 'not_in') ? 'e.g. error, fatal (comma separated)'
          : 'e.g. pattern or regex'
        }
        className="w-full text-xs"
        type={typeInfo?.category === 'number' ? 'number' : 'text'}
      />
    </div>
  )
}
