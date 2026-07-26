import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

// Super-admin runs on its OWN origin (:3002), independent from the coordinator
// PWA (:3000), the tenant admin/staff console (:3001), and the student portal (:3003).
// Offline-first: app shell precached + API GETs cached (NetworkFirst) so it opens
// and shows last data offline, refreshing to current data when online.
export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      injectRegister: 'auto',
      manifest: {
        name: 'QAAT Super-Admin', short_name: 'QAAT SA',
        theme_color: '#1e293b', background_color: '#f1f5f9', display: 'standalone', start_url: '/',
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
  server: { port: 3002, host: true },
  build: { target: 'es2020', sourcemap: true },
})
