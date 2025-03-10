FROM golang:alpine

RUN apk add libc6-compat
COPY customer . 
RUN chmod +x customer

ENV MYSQL_USER=admin
ENV MYSQL_PASSWORD=hLW6|O3CSOG>Ig3[$)r3DOg3>~0V
ENV MYSQL_HOST=database-1.cvmmuwuem8je.ap-northeast-2.rds.amazonaws.com
ENV MYSQL_PORT=3306
ENV MYSQL_DBNAME=dev
ENV REDIS_HOST=ap-test.yjes8h.ng.0001.apn2.cache.amazonaws.com
ENV AWS_REGION=ap-northeast-2

EXPOSE 8080
CMD ["./customer"]

