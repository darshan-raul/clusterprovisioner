FROM node:20-alpine AS builder

WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install

COPY . .
RUN npm run build

FROM nginx:alpine
# Copy built files to nginx
COPY --from=builder /app/dist /usr/share/nginx/html
# Setup proxy pass for api
RUN echo 'server { \
    listen 80; \
    location / { root /usr/share/nginx/html; index index.html; try_files $uri $uri/ /index.html; } \
    location /api/ { proxy_pass http://orchestrator:8080/api/; } \
}' > /etc/nginx/conf.d/default.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
