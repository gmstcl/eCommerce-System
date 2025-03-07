FROM golang:alpine       

RUN apk add libc6-compat                                                                                               
COPY product . 
RUN chmod +x product

ENV MYSQL_USER=root                                                                                                     
ENV MYSQL_PASSWORD=Skill53##                                                                                            
ENV MYSQL_HOST=database-1.cvmmuwuem8je.ap-northeast-2.rds.amazonaws.com
ENV MYSQL_PORT=3306                                                                                                     
ENV MYSQL_DBNAME=dev
ENV REDIS_HOST=clustercfg.dev.yjes8h.apn2.cache.amazonaws.com
ENV REDIS_PORT=6582                                                                                                     
ENV AWS_REGION=ap-northeast-2                                                                                           

EXPOSE 8080
  
CMD ["./product"] 
