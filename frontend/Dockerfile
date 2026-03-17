FROM node:22-alpine AS build
ARG VITE_BASE=/
WORKDIR /app
COPY package.json ./
RUN npm install
COPY . .
RUN VITE_BASE=$VITE_BASE npm run build

FROM nginx:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
