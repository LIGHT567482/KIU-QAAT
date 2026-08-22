FROM nginx:1.27-alpine
COPY infra/nginx/qaat.conf.template /etc/nginx/qaat.conf.template
COPY infra/nginx/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
EXPOSE 80
ENTRYPOINT ["/docker-entrypoint.sh"]
