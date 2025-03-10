package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-redis/redis/v8"
)

var db *sqlx.DB
var rdsClient *rdsdata.Client
var redisClient *redis.Client
var ctx = context.Background()

var (
	mysqlUser     = os.Getenv("MYSQL_USER")
	mysqlPassword = os.Getenv("MYSQL_PASSWORD")
	mysqlHost     = os.Getenv("MYSQL_HOST")
	mysqlPort     = os.Getenv("MYSQL_PORT")
	mysqlDbName   = os.Getenv("MYSQL_DBNAME")
	region        = os.Getenv("AWS_REGION")
	redisHost     = os.Getenv("REDIS_HOST")
	redisUseTLS   = os.Getenv("REDIS_USE_TLS") == "true"
)

type Customer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Gender string `json:"gender"`
}

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		log.Fatalf("unable to load AWS SDK config: %v", err)
	}
	rdsClient = rdsdata.NewFromConfig(cfg)

	var redisOptions *redis.Options
	commonOptions := &redis.Options{
		Addr:         fmt.Sprintf("%s:6379", redisHost),
		DB:           0,
		DialTimeout:  30 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		PoolSize:     20,
		MinIdleConns: 5,
		MaxRetries:   3,
	}
	if redisUseTLS {
		commonOptions.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}
	redisOptions = commonOptions

	redisClient = redis.NewClient(redisOptions)

	var pingErr error
	for i := 0; i < 3; i++ {
		_, pingErr = redisClient.Ping(ctx).Result()
		if pingErr == nil {
			log.Printf("Successfully connected to Redis on attempt %d", i+1)
			break
		}
		log.Printf("Redis ping attempt %d failed: %v", i+1, pingErr)
		time.Sleep(2 * time.Second)
	}
	if pingErr != nil {
		log.Fatalf("failed to connect to Redis after retries: %v", pingErr)
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
	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "customer ID is required"})
		return
	}

	customerData, err := getFromDB(customerID)
	if err != nil {
		log.Printf("Failed to fetch from DB for customerID %s: %v", customerID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch from DB"})
		return
	}

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

	c.JSON(http.StatusCreated, gin.H{"message": "Customer created successfully"})
}

func getFromDB(customerID string) (*Customer, error) {
	val, err := redisClient.Get(ctx, customerID).Result()
	if err == nil {
		parts := strings.Split(val, ",")
		if len(parts) == 2 {
			return &Customer{
				ID:     customerID,
				Name:   parts[0],
				Gender: parts[1],
			}, nil
		}
	}

	sqlQuery := "SELECT id, name, gender FROM customers WHERE id = ?"
	var customer Customer
	if err := db.Get(&customer, sqlQuery, customerID); err != nil {
		log.Printf("Error fetching from DB for customerID %s: %v", customerID, err)
		return nil, err
	}

	if err := redisClient.Set(ctx, customer.ID, fmt.Sprintf("%s,%s", customer.Name, customer.Gender), 24*time.Hour).Err(); err != nil {
		log.Printf("Failed to cache to Redis for customerID %s: %v", customer.ID, err)
	}

	return &customer, nil
}

func saveToDB(customer *Customer) error {
	sqlQuery := `INSERT INTO customers (id, name, gender) VALUES (?, ?, ?)`
	if _, err := db.Exec(sqlQuery, customer.ID, customer.Name, customer.Gender); err != nil {
		log.Printf("Error saving to DB for customerID %s: %v", customer.ID, err)
		return err
	}
	log.Printf("Successfully saved to DB for customerID %s", customer.ID)

	if err := redisClient.Set(ctx, customer.ID, fmt.Sprintf("%s,%s", customer.Name, customer.Gender), 24*time.Hour).Err(); err != nil {
		log.Printf("Failed to cache to Redis for customerID %s: %v", customer.ID, err)
		return err
	}
	log.Printf("Successfully cached to Redis for customerID %s", customer.ID)
	return nil
}

