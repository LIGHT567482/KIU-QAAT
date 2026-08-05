import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

// Offline-first: the app shell is precached so the dashboard loads with no network,
// and API GETs are cached (NetworkFirst) so it shows the last data offline and
// refreshes to current data when back online.
export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      injectRegister: 'auto',
      manifest: {
        // Named and iconed like every other surface. The manifest carried NO icons at all, so
        // installing this to a home screen produced a blank tile — and the theme colour was a
        // slate grey that belonged to nothing, rather than the institution's green.
        name: 'KIU QAAT', short_name: 'KIU QAAT',
        description: 'Kampala International University — Quality Assurance Attendance Tracker',
        theme_color: '#1a7a3f', background_color: '#f0fdf4', display: 'standalone', start_url: '/',
        icons: [
          { src: '/icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: '/icon-512.png', sizes: '512x512', type: 'image/png' },
          // "maskable" lets Android crop the mark to its own shape without clipping the logo.
          { src: '/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2}'],
        navigateFallback: '/index.html',
        cleanupOutdatedCaches: true,
        runtimeCaching: [
          {
            urlPattern: ({ url }) => url.pathname.startsWith('/api/v1/'),
            handler: 'NetworkFirst',
            options: {
              cacheName: 'qaat-api-cache',
              networkTimeoutSeconds: 4,
              expiration: { maxEntries: 300, maxAgeSeconds: 7 * 86400 },
              cacheableResponse: { statuses: [0, 200] },
            },
          },
        ],
      },
    }),
  ],
  server: { port: 3001, host: true },
  build: { target: 'es2020', sourcemap: true },
})
