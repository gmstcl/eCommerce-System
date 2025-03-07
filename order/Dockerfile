FROM golang:alpine       

RUN apk add libc6-compat                                                                                               
COPY order . 
RUN chmod +x order

ENV AWS_REGION=ap-northeast-2                                                                                                                                                                                                                       
EXPOSE 8080                                                                                                                                                                                                                                     
CMD ["./order"] 
