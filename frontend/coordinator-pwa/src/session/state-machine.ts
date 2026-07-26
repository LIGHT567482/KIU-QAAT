import { createMachine, assign } from 'xstate'

// Session state machine — mirrors the spec in ARCHITECT.md §5 exactly.
// Timers enforced here:
//   Gate-Open QR: 15-min TTL from PENDING_LECTURER → ACTIVE
//   Check-in window: T+120 min from gate_open_time
//   Auto-kill: T+180 min from session initialisation

export type SessionStatus =
  | 'IDLE'
  | 'PENDING_LECTURER'
  | 'ACTIVE'
  | 'CLOSED'
  | 'AUTO_CLOSED'

export interface SessionContext {
  session_id: string | null
  unit_id: string | null
  unit_name: string | null
  venue_id: string | null
  lecturer_id: string | null
  gate_open_time: string | null
  checkin_window_end: string | null
  auto_close_time: string | null
  attendance_count: number
  policy: {
    checkin_window_minutes: number
    auto_kill_minutes: number
  }
}

export type SessionEvent =
  | { type: 'SELECT_UNIT'; unit_id: string; unit_name: string; venue_id: string }
  | { type: 'LECTURER_GATE_OPEN'; lecturer_id: string }
  | { type: 'STUDENT_CHECKED_IN' }
  | { type: 'END_SESSION' }
  | { type: 'CHECKIN_WINDOW_EXPIRED' }
  | { type: 'AUTO_KILL_EXPIRED' }
  | { type: 'RESET' }

const initialContext: SessionContext = {
  session_id: null,
  unit_id: null,
  unit_name: null,
  venue_id: null,
  lecturer_id: null,
  gate_open_time: null,
  checkin_window_end: null,
  auto_close_time: null,
  attendance_count: 0,
  policy: {
    checkin_window_minutes: 120,
    auto_kill_minutes: 180,
  },
}

export const sessionMachine = createMachine(
  {
    id: 'session',
    initial: 'IDLE',
    context: initialContext,
    types: {} as { context: SessionContext; events: SessionEvent },

    states: {
      IDLE: {
        on: {
          SELECT_UNIT: {
            target: 'PENDING_LECTURER',
            actions: assign(({ event }) => ({
              unit_id: event.unit_id,
              unit_name: event.unit_name,
              venue_id: event.venue_id,
              session_id: crypto.randomUUID(),
            })),
          },
        },
      },

      PENDING_LECTURER: {
        // Gate-Open QR shown; waits for lecturer scan.
        // 15-min TTL is enforced in the QR generator, not here.
        on: {
          LECTURER_GATE_OPEN: {
            target: 'ACTIVE',
            actions: assign(({ event, context }) => {
              const now = new Date()
              const checkinEnd = new Date(now.getTime() + context.policy.checkin_window_minutes * 60_000)
              const autoClose = new Date(now.getTime() + context.policy.auto_kill_minutes * 60_000)
              return {
                lecturer_id: event.lecturer_id,
                gate_open_time: now.toISOString(),
                checkin_window_end: checkinEnd.toISOString(),
                auto_close_time: autoClose.toISOString(),
              }
            }),
          },
          RESET: { target: 'IDLE', actions: assign(initialContext) },
        },
      },

      ACTIVE: {
        on: {
          STUDENT_CHECKED_IN: {
            actions: assign(({ context }) => ({
              attendance_count: context.attendance_count + 1,
            })),
          },
          END_SESSION: {
            target: 'CLOSED',
            actions: assign({ coordinator_end_time: new Date().toISOString() } as Partial<SessionContext>),
          },
          CHECKIN_WINDOW_EXPIRED: { target: 'CLOSED' },
          AUTO_KILL_EXPIRED:      { target: 'AUTO_CLOSED' },
        },
      },

      CLOSED: {
        type: 'final',
      },

      AUTO_CLOSED: {
        type: 'final',
      },
    },
  },
)
