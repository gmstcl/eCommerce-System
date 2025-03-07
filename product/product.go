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
    region        = os.Getenv("REGION")
)

type Product struct {
    ID       string `json:"id"`
    Name     string `json:"name"`
    Category string `json:"category"`
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

    router.GET("/v1/product", getProduct)
    router.POST("/v1/product", createProduct)

    router.Run(":8080")
}

func getProduct(c *gin.Context) {
    productID := c.DefaultQuery("id", "")

    productData, err := getFromCache(productID)
    if err != nil {
        log.Printf("Failed to fetch from cache for productID %s: %v", productID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch from cache"})
        return
    }

    if productData != nil {
        c.JSON(http.StatusOK, productData)
        return
    }

    productData, err = getFromDB(productID)
    if err != nil {
        log.Printf("Failed to fetch from DB for productID %s: %v", productID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch from DB"})
        return
    }

    saveToCache(productData)

    c.JSON(http.StatusOK, productData)
}

func createProduct(c *gin.Context) {
    var product Product
    if err := c.ShouldBindJSON(&product); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if err := saveToDB(&product); err != nil {
        log.Printf("Failed to save to DB for productID %s: %v", product.ID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save to DB"})
        return
    }

    saveToCache(&product)

    c.JSON(http.StatusCreated, gin.H{"message": "Product created successfully"})
}

func getFromCache(productID string) (*Product, error) {
    val, err := redisClient.Get(ctx, productID).Result()
    if err == redis.Nil {
        log.Printf("No cache found for productID: %s", productID)
        return nil, nil
    } else if err != nil {
        log.Printf("Error fetching from Redis for productID %s: %v", productID, err)
        return nil, err
    }

    var product Product
    err = json.Unmarshal([]byte(val), &product)
    if err != nil {
        log.Printf("Error unmarshalling data for productID %s: %v", productID, err)
        return nil, err
    }

    return &product, nil
}

func saveToCache(product *Product) {
    data, err := json.Marshal(product)
    if err != nil {
        log.Printf("Failed to marshal product: %v", err)
        return
    }

    err = redisClient.Set(ctx, product.ID, data, 0).Err()
    if err != nil {
        log.Printf("Failed to save to cache for productID %s: %v", product.ID, err)
    } else {
        log.Printf("Successfully saved to cache for productID %s", product.ID)
    }
}

func getFromDB(productID string) (*Product, error) {
    sqlQuery := "SELECT id, name, category FROM product WHERE id = ?"
    var product Product
    err := db.Get(&product, sqlQuery, productID)
    if err != nil {
        log.Printf("Error fetching from DB for productID %s: %v", productID, err)
        return nil, err
    }
    return &product, nil
}

func saveToDB(product *Product) error {
    sqlQuery := `INSERT INTO product (id, name, category) VALUES (?, ?, ?)`
    _, err := db.Exec(sqlQuery, product.ID, product.Name, product.Category)
    if err != nil {
        log.Printf("Error saving to DB for productID %s: %v", product.ID, err)
        return err
    }
    log.Printf("Successfully saved to DB for productID %s", product.ID)
    return nil
}
