/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        bg: '#f6f7f9',
        panel: '#ffffff',
        ink: '#111827',
        muted: '#6b7280',
        line: '#e5e7eb',
        accent: '#2563eb',
        good: '#0f9f6e',
        warn: '#b7791f',
        bad: '#dc2626',
      },
      fontFamily: {
        sans: ['Inter', 'ui-sans-serif', 'system-ui', '-apple-system', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'],
      },
    },
  },
  plugins: [],
}
