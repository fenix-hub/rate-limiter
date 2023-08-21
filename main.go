package main

import (
    "os"
    "strconv"
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/go-redis/redis/v8"
    "golang.org/x/net/context"
)

type limit struct {
    Parameter string `json:"parameter" redis:"p"`
    Max int `json:"max" redis:"m"`
    Increment int `json:"increment" redis:"i"`
    Current int `json:"current" redis:"c"`
}

type setupRequest struct {
    Limits []limit `json:"limits"`
}

type countRequest struct {
}

var redisClient *redis.Client
var namespace = "apikey:"

func main() {

    ctx := context.Background()
    redisClient = redis.NewClient(&redis.Options{
        Addr: os.Getenv("REDIS_HOST") + ":" +os.Getenv("REDIS_PORT"),
    })

    // Ping Redis to check the connection
    res, err := redisClient.Ping(ctx).Result()
    if err != nil || res != "PONG" {
        panic("could not connect to redis")
        return
    }

    router := gin.Default()
    router.POST("/setup/:tokenId", setup)
    router.GET("/check/:tokenId", check)
    router.POST("/count/:tokenId", count)

    router.Run("localhost:8080")
}

func check(c *gin.Context) {

    ctx := context.Background()

    tokenId := c.Param("tokenId")
    limitsIndex := namespace + tokenId + ":limits"
    limits, err := redisClient.Get(ctx, limitsIndex).Result()

    if err != nil {
        c.IndentedJSON(http.StatusNotFound, err.Error())
        return
    }

    limitsAmount, err := strconv.Atoi(limits)
    if err != nil {
        c.IndentedJSON(http.StatusInternalServerError, err.Error())
        return
    }

    for i := 0; i < limitsAmount; i++ {
        hash := limitsIndex + ":" + strconv.Itoa(i)
        res := redisClient.HGetAll(ctx, hash)
        if res.Err() != nil || len(res.Val()) == 0 {
            c.Status(http.StatusInternalServerError)
            return
        }
        var limit limit
        if err := res.Scan(&limit); err != nil {
            c.IndentedJSON(http.StatusInternalServerError, err.Error())
            return
        }
        c.Header("X-RateLimit-" + limit.Parameter, strconv.Itoa(limit.Max))
        c.Header("X-RateLimit-Remaining-" + limit.Parameter, strconv.Itoa(limit.Max - limit.Current))
        // c.Header("X-RateLimit-Reset-" + limit.Parameter, strconv.Itoa(limit.Increment))
        if limit.Current >= limit.Max {
            c.IndentedJSON(http.StatusTooManyRequests, "You have exceeded the limit of " + limit.Parameter + " requests")
            return
        }
    }

    c.Status(http.StatusNoContent)
}

func count(c *gin.Context) {

    ctx := context.Background()

    tokenId := c.Param("tokenId")
    limitsIndex := namespace + tokenId + ":limits"
    limits, err := redisClient.Get(ctx, limitsIndex).Result()

    if err != nil {
        c.IndentedJSON(http.StatusNotFound, err.Error())
        return
    }

    limitsAmount, err := strconv.Atoi(limits)
    if err != nil {
        c.IndentedJSON(http.StatusInternalServerError, err.Error())
        return
    }

    for i := 0; i < limitsAmount; i++ {
        hash := limitsIndex + ":" + strconv.Itoa(i)
        res := redisClient.HGetAll(ctx, hash)
        if res.Err() != nil {
            c.IndentedJSON(http.StatusInternalServerError, err.Error())
            return
        }
        var limit limit
        if err := res.Scan(&limit); err != nil {
            c.IndentedJSON(http.StatusInternalServerError, err.Error())
            return
        }

        if limit.Parameter == "CALL" {
            redisClient.HIncrBy(ctx, hash, "c", int64(limit.Increment))
        }
    }

    c.Status(http.StatusNoContent)
}

func setup(c *gin.Context) {
    ctx := context.Background()

    // Parse the body into our resource
    var body setupRequest
    if err := c.BindJSON(&body); err != nil {
        c.IndentedJSON(http.StatusBadRequest, err.Error())
        return
    }

    tokenId := c.Param("tokenId")
    limits := body.Limits
    limitsIndex := namespace + tokenId + ":limits"

    for i, limit := range limits {
        hash := limitsIndex + ":" + strconv.Itoa(i)
        redisClient.HSet(ctx, hash, "p", limit.Parameter, "c", 0, "m", limit.Max, "i", limit.Increment)
    }

    redisClient.Set(ctx, limitsIndex, len(limits), 0)

    c.Status(http.StatusNoContent)
}