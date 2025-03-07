package main

import (
    "context"
    "log"
    "net/http"
    "os"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "github.com/gin-gonic/gin"
)

var (
    region       = os.Getenv("AWS_REGION")
    dynamoClient *dynamodb.Client
    ctx          = context.Background()
)

type Order struct {
    ID        string `json:"id"`
    CustomerID string `json:"customerid"`
    ProductID  string `json:"productid"`
}

func init() {
    cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
    if err != nil {
        log.Fatalf("unable to load SDK config, %v", err)
    }
    dynamoClient = dynamodb.NewFromConfig(cfg)
}

func main() {
    router := gin.Default()

    router.GET("/v1/order", getOrder)
    router.POST("/v1/order", createOrder)

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

    if err := saveOrderToDynamoDB(&order); err != nil {
        log.Printf("Failed to save order to DynamoDB for orderID %s: %v", order.ID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save order"})
        return
    }

    c.JSON(http.StatusCreated, gin.H{"message": "Order created successfully"})
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
