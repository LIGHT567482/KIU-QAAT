export default function Unauthorized() {
  return (
    <div style={{ padding: 40, fontFamily: 'system-ui' }}>
      <h2>Access Denied</h2>
      <p>Your role does not have permission to view this page.</p>
    </div>
  )
}
