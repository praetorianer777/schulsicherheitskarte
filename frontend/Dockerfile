FROM node:26-alpine AS build
WORKDIR /src
# Dependencies change far less often than the source, so they get their own layer.
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

# The unprivileged image listens on 8080 as a non-root user, so nothing here
# needs a privileged port or a root process.
FROM nginxinc/nginx-unprivileged:1.29-alpine AS web
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 8080
