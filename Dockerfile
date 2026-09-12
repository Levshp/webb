FROM nginx:alpine

COPY . /usr/share/nginx/html

COPY styles.css /usr/share/nginx/html/styles.css

EXPOSE 80
