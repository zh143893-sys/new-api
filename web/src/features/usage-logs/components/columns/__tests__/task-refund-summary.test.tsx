/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import { formatLogQuota, formatTimestampToDate } from '@/lib/format'

import {
  formatAdminDiagnostic,
  TaskRefundSummary,
} from '../task-logs-columns'

describe('task refund summary', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      'Refunded {{amount}}': 'Refunded {{amount}}',
      'Refund Time': 'Refund Time',
    })
  })

  test('shows the refunded quota and refund time for a failed task', () => {
    const refundedQuota = 8_500_000
    const refundedAt = 1_787_643_904

    render(
      <TaskRefundSummary
        refundedQuota={refundedQuota}
        refundedAt={refundedAt}
      />
    )

    expect(
      screen.getByText(`Refunded ${formatLogQuota(refundedQuota)}`)
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        `Refund Time: ${formatTimestampToDate(refundedAt, 'seconds')}`
      )
    ).toBeInTheDocument()
  })

  test('formats only the structured administrator diagnostic', () => {
    const text = formatAdminDiagnostic({
      id: 1,
      user_id: 1,
      platform: 'video',
      task_id: 'task_public123',
      action: 'GENERATE',
      channel_id: 0,
      submit_time: 1,
      status: 'FAILURE',
      admin_diagnostic: {
        code: 'UPSTREAM_RATE_LIMITED',
        stage: 'upstream_poll',
        summary: '上游线路触发限流或队列繁忙',
        action: '降低该线路并发或等待后重试',
        upstream_http_status: 429,
        retryable: true,
      },
    })

    expect(text).toContain('上游线路触发限流或队列繁忙')
    expect(text).toContain('上游 HTTP 状态：429')
    expect(text).toContain('可重试：是')
    expect(text).not.toContain('provider')
  })
})
