import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['-apple-system', 'BlinkMacSystemFont', '"SF Pro Display"', '"PingFang SC"', 'sans-serif'],
      },
      colors: {
        bg: '#FFFFFF',
        'bg-subtle': '#F5F5F7',
        ink: '#1D1D1F',
        'ink-soft': '#86868B',
        accent: '#0071E3',
        ok: '#34C759',
        warn: '#FF9F0A',
        bad: '#FF3B30',
        'border-soft': '#D2D2D7',
      },
    },
  },
  plugins: [],
} satisfies Config
