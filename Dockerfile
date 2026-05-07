FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY js-api /app/js-api
RUN chmod +x /app/js-api

ENV LISTEN_ADDR=0.0.0.0
ENV LISTEN_PORT=80
ENV DB_HOST=IP/FQDN
ENV DB_PORT=3306
ENV DB_NAME=CHANGE-ME
ENV DB_USER=CHANGE-ME
ENV DB_PASSWORD=CHANGE-ME
ENV DB_DRIVER=mysql

EXPOSE 80

CMD ["/app/js-api"]