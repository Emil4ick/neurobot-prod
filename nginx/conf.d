server {
    listen 80;
    server_name yourneuro.ru www.yourneuro.ru;

    location / {
        return 301 https://$host$request_uri;
    }
}

server {
    listen 443 ssl;
    server_name yourneuro.ru www.yourneuro.ru;

    ssl_certificate /etc/nginx/ssl/yourneuro.ru/yourneuro.ru.crt;
    ssl_certificate_key /etc/nginx/ssl/yourneuro.ru/yourneuro.ru.key;
    ssl_trusted_certificate /etc/nginx/ssl/yourneuro.ru/ca.crt;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;
    ssl_ciphers 'EECDH+AESGCM:EDH+AESGCM:AES256+EECDH:AES256+EDH';
    ssl_session_timeout 1d;
    ssl_session_cache shared:SSL:50m;
    ssl_stapling on;
    ssl_stapling_verify on;

    add_header Strict-Transport-Security "max-age=63072000; includeSubdomains; preload";
    add_header X-Content-Type-Options nosniff;
    add_header X-Frame-Options DENY;
    add_header X-XSS-Protection "1; mode=block";

    # Основной сайт
    location / {
        root /usr/share/nginx/html;
        index index.html;
        try_files $uri $uri/ /index.html;
    }

    # Webhook для Telegram
    location /webhook {
        proxy_pass http://webhook:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Mini App (WebApp)
    location /webapp {
        root /usr/share/nginx/html;
        try_files $uri $uri/ /webapp/index.html;
    }

    # API для WebApp
    location /api {
        proxy_pass http://webhook:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}