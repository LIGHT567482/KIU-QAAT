import RecordTabs from '../../components/RecordTabs'
import UPanelRecords from './UPanelRecords'

export default function UPanelAttendance() {
  return (
    <RecordTabs title="U-Panel Attendance" tabs={[
      {
        id: 'student',
        label: 'Students',
        hint: 'Class check-ins from U-Panel, stored in QAAT and used as student-attendance data.',
        render: () => <UPanelRecords kind="student" />,
      },
      {
        id: 'lecturer',
        label: 'Lecturers',
        hint: 'Lecture sittings and lecturer sign-ins from U-Panel, stored in QAAT.',
        render: () => <UPanelRecords kind="lecturer" />,
      },
      {
        id: 'admin',
        label: 'Admin / staff',
        hint: 'Campus arrival and departure from U-Panel. Also written into employee attendance.',
        render: () => <UPanelRecords kind="admin" />,
      },
    ]} />
  )
}
