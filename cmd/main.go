package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sonyamoonglade/s3-yandex-go/s3yandex"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {

	v1 := gin.Default()

	if err := godotenv.Load(".env"); err != nil {
		log.Fatal(err.Error())
	}

	envProvider := s3yandex.NewEnvCredentialsProvider()

	yandexConfig := s3yandex.YandexS3Config{
		Owner:  os.Getenv("BUCKET_OWNER_ID"),
		Bucket: "zharpizza-bucket",
		Debug:  true,
	}
	client := s3yandex.NewYandexS3Client(envProvider, yandexConfig)

	h := NewHandler(client)

	corsMiddleware := cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowCredentials: true,
		AllowFiles:       true,
		AllowHeaders: []string{
			"Content-type",
			"Content-length",
			"x-file-ext",
			"x-file-name",
		},
	})

	v1.Use(corsMiddleware)

	v1.POST("service/put", h.UploadFile)

	log.Fatal(v1.Run(":5001"))

}

type Handler struct {
	client *s3yandex.YandexS3Client
}

func NewHandler(client *s3yandex.YandexS3Client) *Handler {
	return &Handler{client: client}
}

func (h *Handler) UploadFile(c *gin.Context) {
	fileBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(500, "error reading body(file buffer)")
		return
	}

	fileName := c.GetHeader("x-file-name")
	fileExt := c.GetHeader("x-file-ext")
	fileDest := c.GetHeader("x-destination")
	sessionId := c.GetHeader("x-session-id")

	isAuth, err := CheckAuthorization(sessionId)
	if !isAuth {
		c.Status(401)
		return
	}

	fileNameWithExtension := fmt.Sprintf("%s.%s", fileName, fileExt)

	err = h.client.PutFileWithBytes(context.TODO(), &s3yandex.PutFileWithBytesInput{
		ContentType: s3yandex.ImagePNG,
		FileName:    fileNameWithExtension,
		Destination: fileDest,
		FileBytes:   &fileBytes,
	})
	if err != nil {
		fmt.Println(err.Error())
		c.JSON(500, gin.H{
			"ok": false,
		})
		return
	}

	c.JSON(201, gin.H{
		"ok": true,
	})
	return
}

type AuthResponse struct {
	Ok bool `json:"ok,omitempty"`
}

func CheckAuthorization(sessionId string) (bool, error) {

	mainServiceHost := os.Getenv("MAIN_SERVICE_URL")
	finalURL := mainServiceHost + "/api/v1/users/service/me"

	headers := map[string][]string{
		"x-session-id": {sessionId},
	}

	r, _ := http.NewRequest(http.MethodGet, finalURL, nil)
	r.Header = headers

	client := http.Client{}

	res, err := client.Do(r)
	if err != nil {
		return false, err
	}

	var input AuthResponse

	b, err := io.ReadAll(res.Body)
	err = json.Unmarshal(b, &input)
	if err != nil {
		return false, err
	}

	return input.Ok, nil

}
