import { useState, type CSSProperties, type InputHTMLAttributes } from 'react'

/**
 * A drop-in replacement for <input type="password"> with a built-in Show/Hide
 * toggle, so a user can reveal what they typed. Forwards every input prop
 * (value, onChange, placeholder, style, autoComplete, required…). `wrapperStyle`
 * styles the positioning wrapper — pass e.g. { flex: 1 } inside a flex row.
 */
export default function PasswordInput(
  { style, wrapperStyle, ...props }:
    InputHTMLAttributes<HTMLInputElement> & { wrapperStyle?: CSSProperties },
) {
  const [show, setShow] = useState(false)
  return (
    <div style={{ position: 'relative', display: 'block', ...wrapperStyle }}>
      <input
        {...props}
        type={show ? 'text' : 'password'}
        style={{ ...(style as CSSProperties), width: '100%', paddingRight: 62, boxSizing: 'border-box' }}
      />
      <button
        type="button"
        onClick={() => setShow(s => !s)}
        tabIndex={-1}
        aria-label={show ? 'Hide password' : 'Show password'}
        style={{
          position: 'absolute', right: 8, top: '50%', transform: 'translateY(-50%)',
          border: 'none', background: 'none', cursor: 'pointer', fontSize: 12,
          fontWeight: 600, color: '#64748b', padding: 4,
        }}
      >
        {show ? 'Hide' : 'Show'}
      </button>
    </div>
  )
}
