# Production image for the super-admin frontend.
# The app is built LOCALLY (pnpm build) with VITE_API_URL baked in; this image
# just serves the prebuilt dist/ via nginx. (Avoids pnpm-in-Docker tarball bugs.)
FROM nginx:alpine
COPY dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 3002
