FROM node:22-alpine AS build
WORKDIR /app
COPY web_ui/package*.json ./
RUN npm ci
COPY web_ui .
RUN npm run build
FROM nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY web_ui/nginx.conf /etc/nginx/conf.d/default.conf

