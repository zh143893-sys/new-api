/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { ColumnDef } from '@tanstack/react-table'
import { Music, Play } from 'lucide-react'
/* eslint-disable react-refresh/only-export-components */
import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { formatLogQuota, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import { TASK_ACTIONS, TASK_STATUS } from '../../constants'
import { getTaskPreviewUrl } from '../../api'
import { taskActionMapper, taskStatusMapper } from '../../lib/mappers'
import type { TaskLog } from '../../types'
import {
  AudioPreviewDialog,
  type AudioClip,
} from '../dialogs/audio-preview-dialog'
import { FailReasonDialog } from '../dialogs/fail-reason-dialog'
import { useUsageLogsContext } from '../usage-logs-provider'
import {
  createDurationColumn,
  createChannelColumn,
  createProgressColumn,
} from './column-helpers'

function parseTaskData(data: unknown): unknown[] {
  if (Array.isArray(data)) return data
  if (typeof data === 'string') {
    try {
      const parsed = JSON.parse(data)
      return Array.isArray(parsed) ? parsed : []
    } catch {
      return []
    }
  }
  return []
}

function AudioPreviewCell({ log }: { log: TaskLog }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const clips = useMemo(() => {
    const data = parseTaskData(log.data)
    return data.filter(
      (c) =>
        c && typeof c === 'object' && (c as Record<string, unknown>).audio_url
    )
  }, [log.data])

  if (clips.length === 0) return null

  return (
    <>
      <button
        type='button'
        className='group flex items-center gap-1 text-left text-xs'
        onClick={() => setOpen(true)}
      >
        <Music className='text-muted-foreground size-3' />
        <span className='text-foreground leading-snug group-hover:underline'>
          {t('Click to preview audio')}
        </span>
      </button>
      <AudioPreviewDialog
        open={open}
        onOpenChange={setOpen}
        clips={clips as AudioClip[]}
      />
    </>
  )
}

export function formatAdminDiagnostic(log: TaskLog): string {
  const diagnostic = log.admin_diagnostic
  if (!diagnostic) return log.fail_reason || ''

  const lines = [
    `原因：${diagnostic.summary}`,
    `故障代码：${diagnostic.code}`,
    `发生环节：${diagnostic.stage}`,
  ]
  if (diagnostic.upstream_http_status) {
    lines.push(`上游 HTTP 状态：${diagnostic.upstream_http_status}`)
  }
  lines.push(`建议处理：${diagnostic.action}`)
  lines.push(`可重试：${diagnostic.retryable ? '是' : '否'}`)
  if (diagnostic.historical) {
    lines.push('说明：历史任务未保存更细的结构化诊断')
  }
  return lines.join('\n')
}

export function TaskVideoPreview({ log }: { log: TaskLog }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [previewUrl, setPreviewUrl] = useState('')

  async function openPreview() {
    setLoading(true)
    try {
      const result = await getTaskPreviewUrl(log.task_id)
      const url = result.data?.url
      if (!result.success || !url) {
        toast.error(result.message || t('Preview is unavailable'))
        return
      }
      setPreviewUrl(url)
      setOpen(true)
    } catch {
      toast.error(t('Preview is unavailable'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      <button
        type='button'
        className='group flex items-center gap-1 text-left text-xs disabled:opacity-50'
        disabled={loading}
        onClick={openPreview}
      >
        <Play className='text-muted-foreground size-3' />
        <span className='text-foreground leading-snug group-hover:underline'>
          {loading ? t('Loading...') : t('Click to preview video')}
        </span>
      </button>
      <Dialog
        open={open}
        onOpenChange={(next) => {
          setOpen(next)
          if (!next) setPreviewUrl('')
        }}
        title={t('Video Preview')}
        description={log.task_id}
        contentClassName='sm:max-w-4xl'
        contentHeight='auto'
      >
        {previewUrl ? (
          <video
            className='max-h-[70vh] w-full rounded-md bg-black'
            controls
            preload='metadata'
            src={previewUrl}
          />
        ) : null}
      </Dialog>
    </>
  )
}

export function TaskRefundSummary(props: {
  refundedQuota: number
  refundedAt?: number
}) {
  const { t } = useTranslation()

  return (
    <div className='flex flex-col items-start gap-0.5'>
      <StatusBadge
        label={t('Refunded {{amount}}', {
          amount: formatLogQuota(props.refundedQuota),
        })}
        variant='success'
        size='sm'
        copyable={false}
      />
      {props.refundedAt ? (
        <span className='text-muted-foreground/70 text-[11px] tabular-nums'>
          {t('Refund Time')}:{' '}
          {formatTimestampToDate(props.refundedAt, 'seconds')}
        </span>
      ) : null}
    </div>
  )
}

export function useTaskLogsColumns(isAdmin: boolean): ColumnDef<TaskLog>[] {
  const { t } = useTranslation()
  const columns: ColumnDef<TaskLog>[] = [
    {
      accessorKey: 'submit_time',
      header: t('Submit Time'),
      cell: ({ row }) => {
        const log = row.original
        const submitTime = row.getValue('submit_time') as number

        return (
          <div className='flex min-w-0 flex-col gap-0.5'>
            <span className='truncate font-mono text-xs tabular-nums'>
              {formatTimestampToDate(submitTime, 'seconds')}
            </span>
            {log.finish_time ? (
              <span className='text-muted-foreground/60 truncate font-mono text-[11px] tabular-nums'>
                {formatTimestampToDate(log.finish_time, 'seconds')}
              </span>
            ) : (
              <span className='text-muted-foreground/50 text-[11px]'>-</span>
            )}
          </div>
        )
      },
      size: 180,
    },
  ]

  if (isAdmin) {
    columns.push(createChannelColumn<TaskLog>({ headerLabel: t('Channel') }), {
      id: 'user',
      header: t('User'),
      accessorFn: (row) => row.username || row.user_id,
      cell: function UserCell({ row }) {
        const { sensitiveVisible, setSelectedUserId, setUserInfoDialogOpen } =
          useUsageLogsContext()
        const log = row.original
        const displayName = log.username || String(log.user_id || '?')

        return (
          <button
            type='button'
            className='flex items-center gap-1.5 text-left'
            onClick={(e) => {
              e.stopPropagation()
              setSelectedUserId(log.user_id)
              setUserInfoDialogOpen(true)
            }}
          >
            <Avatar className='ring-border/60 size-6 ring-1 max-sm:hidden'>
              <AvatarFallback
                className={cn(
                  'text-[11px] font-semibold',
                  !sensitiveVisible && 'bg-muted text-muted-foreground'
                )}
                style={
                  sensitiveVisible ? getUserAvatarStyle(displayName) : undefined
                }
              >
                {sensitiveVisible ? getUserAvatarFallback(displayName) : '•'}
              </AvatarFallback>
            </Avatar>
            <span className='text-muted-foreground truncate text-sm hover:underline'>
              {sensitiveVisible ? displayName : '••••'}
            </span>
          </button>
        )
      },
    })
  }

  columns.push(
    {
      accessorKey: 'task_id',
      header: t('Task ID'),
      cell: ({ row }) => {
        const log = row.original
        const taskId = row.getValue('task_id') as string
        if (!taskId) {
          return <span className='text-muted-foreground/60 text-xs'>-</span>
        }
        return (
          <div className='flex max-w-[170px] flex-col gap-0.5'>
            <StatusBadge
              label={taskId}
              copyText={taskId}
              variant='neutral'
              size='sm'
              className='border-border/60 bg-muted/30 !text-foreground max-w-full truncate rounded-md border px-1.5 py-0.5 font-mono'
            />
            <span className='text-muted-foreground/60 truncate text-[11px]'>
              {t(log.platform)} · {t(taskActionMapper.getLabel(log.action))}
            </span>
          </div>
        )
      },
      meta: { mobileTitle: true },
    },
    createDurationColumn<TaskLog>({
      submitTimeKey: 'submit_time',
      finishTimeKey: 'finish_time',
      unit: 'seconds',
      headerLabel: t('Duration'),
      warningThresholdSec: 300,
    }),
    {
      accessorKey: 'status',
      header: t('Status'),
      cell: ({ row }) => {
        const status = row.getValue('status') as string
        return (
          <StatusBadge
            label={t(taskStatusMapper.getLabel(status, status || 'Submitting'))}
            variant={taskStatusMapper.getVariant(status)}
            size='sm'
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
    },
    createProgressColumn<TaskLog>({ headerLabel: t('Progress') }),
    {
      accessorKey: 'fail_reason',
      header: t('Details'),
      cell: function DetailsCell({ row }) {
        const log = row.original
        const failReason = row.getValue('fail_reason') as string
        const status = log.status
        const refundedQuota = log.refunded_quota ?? 0
        const [dialogOpen, setDialogOpen] = useState(false)
        const diagnosticText = isAdmin ? formatAdminDiagnostic(log) : ''

        const isSunoSuccess =
          log.platform === 'suno' && status === TASK_STATUS.SUCCESS
        if (isSunoSuccess) {
          const data = parseTaskData(log.data)
          if (
            data.some(
              (c) =>
                c &&
                typeof c === 'object' &&
                (c as Record<string, unknown>).audio_url
            )
          ) {
            return <AudioPreviewCell log={log} />
          }
        }

        if (!failReason && !diagnosticText && refundedQuota <= 0) {
          return <span className='text-muted-foreground/60 text-xs'>-</span>
        }

        return (
          <div className='flex max-w-[240px] flex-col items-start gap-1'>
            {refundedQuota > 0 ? (
              <TaskRefundSummary
                refundedQuota={refundedQuota}
                refundedAt={log.refunded_at}
              />
            ) : null}
            {failReason || diagnosticText ? (
              <>
                <button
                  type='button'
                  className='group flex max-w-full items-center gap-1 text-left text-xs'
                  onClick={() => setDialogOpen(true)}
                  title={t('Click to view full error message')}
                >
                  <span className='truncate leading-snug text-red-600 group-hover:underline dark:text-red-400'>
                    {log.admin_diagnostic?.summary || failReason}
                  </span>
                </button>
                <FailReasonDialog
                  failReason={diagnosticText || failReason}
                  open={dialogOpen}
                  onOpenChange={setDialogOpen}
                />
              </>
            ) : null}
          </div>
        )
      },
      size: 240,
      maxSize: 260,
    }
  )

  if (isAdmin) {
    columns.push({
      id: 'preview',
      header: t('Preview'),
      cell: ({ row }) => {
        const log = row.original
        const isVideoTask =
          log.action === TASK_ACTIONS.GENERATE ||
          log.action === TASK_ACTIONS.TEXT_GENERATE ||
          log.action === TASK_ACTIONS.FIRST_TAIL_GENERATE ||
          log.action === TASK_ACTIONS.REFERENCE_GENERATE ||
          log.action === TASK_ACTIONS.REMIX_GENERATE
        if (
          log.status !== TASK_STATUS.SUCCESS ||
          !isVideoTask ||
          !log.preview_available
        ) {
          return <span className='text-muted-foreground/60 text-xs'>-</span>
        }
        return <TaskVideoPreview log={log} />
      },
      size: 150,
    })
  }

  return columns
}
