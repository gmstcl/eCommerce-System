FROM golang:alpine       

RUN apk add libc6-compat                                                                                               
COPY order . 
RUN chmod +x order

ENV AWS_REGION=ap-northeast-2                                                                                           
ENV S3_ACCESS_POINT_ARN=arn:aws:s3::211125622661:accesspoint/mwzo4gh78dnq5.mrap

EXPOSE 8080                                                                                                                                                                                                                                     
CMD ["./order"] 
