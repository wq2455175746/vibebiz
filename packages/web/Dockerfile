FROM node:22-alpine AS build

WORKDIR /app
ARG VITE_CP_BASE=/cp/api/v1
ARG VITE_DP_BASE=/dp/api/v1
ARG VITE_DEMO_LOGIN=true
ENV VITE_CP_BASE=${VITE_CP_BASE} \
    VITE_DP_BASE=${VITE_DP_BASE} \
    VITE_DEMO_LOGIN=${VITE_DEMO_LOGIN}

COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
STOPSIGNAL SIGQUIT

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1/ >/dev/null || exit 1
