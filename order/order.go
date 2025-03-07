package main

import (
    "bytes"
    "context"
    "encoding/json"
    "log"
    "net/http"
    "os"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/gin-gonic/gin"
)

var (
    region           = os.Getenv("AWS_REGION")
    dynamoClient     *dynamodb.Client
    s3Client         *s3.Client
    s3AccessPointARN = os.Getenv("S3_ACCESS_POINT_ARN") 
    ctx              = context.Background()
)

type Order struct {
    ID        string `json:"id"`
    CustomerID string `json:"customerid"`
    ProductID  string `json:"productid"`
}

func init() {
    cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
    if err != nil {
        log.Fatalf("unable to load SDK config, %v", err)
    }
    dynamoClient = dynamodb.NewFromConfig(cfg)
    s3Client = s3.NewFromConfig(cfg)
}

func main() {
    router := gin.Default()

    router.GET("/v1/order", getOrder)
    router.POST("/v1/order", createOrder)
    router.POST("/v1/s3/order", saveOrdersToS3)

    router.Run(":8080")
}

func getOrder(c *gin.Context) {
    orderID := c.DefaultQuery("id", "")

    orderData, err := getOrderFromDynamoDB(orderID)
    if err != nil {
        log.Printf("Failed to fetch order from DynamoDB for orderID %s: %v", orderID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch order"})
        return
    }

    if orderData == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
        return
    }

    c.JSON(http.StatusOK, orderData)
}

func createOrder(c *gin.Context) {
    var order Order
    if err := c.ShouldBindJSON(&order); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if err := saveOrderToDynamoDB(&order); 
    err != nil {
        log.Printf("Failed to save order to DynamoDB for orderID %s: %v", order.ID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save order"})
        return
    }

    c.JSON(http.StatusCreated, gin.H{"message": "Order created successfully"})
}

func saveOrdersToS3(c *gin.Context) {
    orders, err := getAllOrdersFromDynamoDB()
    if err != nil {
        log.Printf("Failed to fetch orders from DynamoDB: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch orders"})
        return
    }

    data, err := json.Marshal(orders)
    if err != nil {
        log.Printf("Failed to marshal orders: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal orders"})
        return
    }

    err = saveDataToS3(data)
    if err != nil {
        log.Printf("Failed to save data to S3: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save data to S3"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Orders saved to S3 successfully"})
}

func getOrderFromDynamoDB(orderID string) (*Order, error) {
    result, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
        TableName: aws.String("order"),
        Key: map[string]types.AttributeValue{ 
            "id": &types.AttributeValueMemberS{
                Value: orderID,
            },
        },
    })
    if err != nil {
        log.Printf("Error fetching order from DynamoDB for orderID %s: %v", orderID, err)
        return nil, err
    }

    if result.Item == nil {
        return nil, nil 
    }

    var order Order
    if id, ok := result.Item["id"].(*types.AttributeValueMemberS); ok {
        order.ID = id.Value
    }
    if customerID, ok := result.Item["customerid"].(*types.AttributeValueMemberS); ok {
        order.CustomerID = customerID.Value
    }
    if productID, ok := result.Item["productid"].(*types.AttributeValueMemberS); ok {
        order.ProductID = productID.Value
    }

    return &order, nil
}

// saveOrderToDynamoDB 함수 추가
func saveOrderToDynamoDB(order *Order) error {
    input := &dynamodb.PutItemInput{
        TableName: aws.String("order"),
        Item: map[string]types.AttributeValue{
            "id": &types.AttributeValueMemberS{
                Value: order.ID,
            },
            "customerid": &types.AttributeValueMemberS{
                Value: order.CustomerID,
            },
            "productid": &types.AttributeValueMemberS{
                Value: order.ProductID,
            },
        },
    }

    _, err := dynamoClient.PutItem(ctx, input)
    if err != nil {
        log.Printf("Error saving order to DynamoDB for orderID %s: %v", order.ID, err)
        return err
    }

    log.Printf("Successfully saved order to DynamoDB for orderID %s", order.ID)
    return nil
}

func getAllOrdersFromDynamoDB() ([]Order, error) {
    var orders []Order
    result, err := dynamoClient.Scan(ctx, &dynamodb.ScanInput{
        TableName: aws.String("order"),
    })
    if err != nil {
        return nil, err
    }

    for _, item := range result.Items {
        var order Order
        if id, ok := item["id"].(*types.AttributeValueMemberS); ok {
            order.ID = id.Value
        }
        if customerID, ok := item["customerid"].(*types.AttributeValueMemberS); ok {
            order.CustomerID = customerID.Value
        }
        if productID, ok := item["productid"].(*types.AttributeValueMemberS); ok {
            order.ProductID = productID.Value
        }
        orders = append(orders, order)
    }

    return orders, nil
}

func saveDataToS3(data []byte) error {
    objectKey := "orders_data.json"

    // S3에 데이터를 저장
    _, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
        Bucket: aws.String(s3AccessPointARN), // 환경변수에서 가져온 ARN 사용
        Key:    aws.String(objectKey),
        Body:   bytes.NewReader(data),
    })
    if err != nil {
        log.Printf("Error saving data to S3: %v", err)
        return err
    }

    log.Printf("Successfully saved data to S3")
    return nil
}

