// main.go 수정
package main

import (
    "context"
    "crypto/tls"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/rdsdata"
    "github.com/gin-gonic/gin"
    "github.com/go-redis/redis/v8"
    "github.com/jmoiron/sqlx"
    _ "github.com/go-sql-driver/mysql"
)

var db *sqlx.DB
var redisClient *redis.Client
var rdsClient *rdsdata.Client
var ctx = context.Background()

var (
    mysqlUser     = os.Getenv("MYSQL_USER")
    mysqlPassword = os.Getenv("MYSQL_PASSWORD")
    mysqlHost     = os.Getenv("MYSQL_HOST")
    mysqlPort     = os.Getenv("MYSQL_PORT")
    mysqlDbName   = os.Getenv("MYSQL_DBNAME")
    redisAddr     = os.Getenv("REDIS_HOST")
    redisPort     = os.Getenv("REDIS_PORT")
    region        = os.Getenv("AWS_REGION")
)

type Customer struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    Gender string `json:"gender"`
}

func init() {
    cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
    if err != nil {
        log.Fatalf("unable to load SDK config, %v", err)
    }
    rdsClient = rdsdata.NewFromConfig(cfg)

    redisClient = redis.NewClient(&redis.Options{
        Addr:     fmt.Sprintf("%s:%s", redisAddr, redisPort),
        TLSConfig: &tls.Config{},  
    })

    checkRedisConnection() 
}

func checkRedisConnection() {
    _, err := redisClient.Ping(ctx).Result()
    if err != nil {
        log.Printf("Redis connection error: %v", err)
    } else {
        log.Println("Connected to Redis successfully")
    }
}

func main() {
    var err error
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", mysqlUser, mysqlPassword, mysqlHost, mysqlPort, mysqlDbName)

    db, err = sqlx.Connect("mysql", dsn)
    if err != nil {
        log.Fatalf("failed to connect to RDS: %v", err)
    }

    router := gin.Default()

    router.GET("/v1/customer", getCustomer)
    router.POST("/v1/customer", createCustomer)

    router.Run(":8080")
}

func getCustomer(c *gin.Context) {
    customerID := c.DefaultQuery("id", "")

    customerData, err := getFromCache(customerID)
    if err != nil {
        log.Printf("Failed to fetch from cache for customerID %s: %v", customerID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch from cache"})
        return
    }

    if customerData != nil {
        c.JSON(http.StatusOK, customerData)
        return
    }

    customerData, err = getFromDB(customerID)
    if err != nil {
        log.Printf("Failed to fetch from DB for customerID %s: %v", customerID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch from DB"})
        return
    }

    saveToCache(customerData)

    c.JSON(http.StatusOK, customerData)
}

func createCustomer(c *gin.Context) {
    var customer Customer
    if err := c.ShouldBindJSON(&customer); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if err := saveToDB(&customer); err != nil {
        log.Printf("Failed to save to DB for customerID %s: %v", customer.ID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save to DB"})
        return
    }

    saveToCache(&customer)

    c.JSON(http.StatusCreated, gin.H{"message": "Customer created successfully"})
}

func getFromCache(customerID string) (*Customer, error) {
    val, err := redisClient.Get(ctx, customerID).Result()
    if err == redis.Nil {
        log.Printf("No cache found for customerID: %s", customerID)
        return nil, nil
    } else if err != nil {
        log.Printf("Error fetching from Redis for customerID %s: %v", customerID, err)
        return nil, err
    }

    var customer Customer
    err = json.Unmarshal([]byte(val), &customer)
    if err != nil {
        log.Printf("Error unmarshalling data for customerID %s: %v", customerID, err)
        return nil, err
    }

    return &customer, nil
}

func saveToCache(customer *Customer) {
    data, err := json.Marshal(customer)
    if err != nil {
        log.Printf("Failed to marshal customer: %v", err)
        return
    }

    err = redisClient.Set(ctx, customer.ID, data, 0).Err()
    if err != nil {
        log.Printf("Failed to save to cache for customerID %s: %v", customer.ID, err)
    } else {
        log.Printf("Successfully saved to cache for customerID %s", customer.ID)
    }
}

func getFromDB(customerID string) (*Customer, error) {
    sqlQuery := "SELECT id, name, gender FROM customers WHERE id = ?"
    var customer Customer
    err := db.Get(&customer, sqlQuery, customerID)
    if err != nil {
        log.Printf("Error fetching from DB for customerID %s: %v", customerID, err)
        return nil, err
    }
    return &customer, nil
}

func saveToDB(customer *Customer) error {
    sqlQuery := `INSERT INTO customers (id, name, gender) VALUES (?, ?, ?)`
    _, err := db.Exec(sqlQuery, customer.ID, customer.Name, customer.Gender)
    if err != nil {
        log.Printf("Error saving to DB for customerID %s: %v", customer.ID, err)
        return err
    }
    log.Printf("Successfully saved to DB for customerID %s", customer.ID)
    return nil
}

