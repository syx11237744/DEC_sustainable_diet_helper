package controllers

import (
	"bytes"
	"encoding/json"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AIController struct {
	DB *gorm.DB
}

type ImageAnalysisRequest struct {
	ImageURL string `json:"image_url" binding:"required"`
}

type IngredientsRequest struct {
	IngredientName string `json:"ingredient_name" binding:"required"`
}

type ImageAnalysisResponse struct {
	Text        string    `json:"text"`
	ProcessedAt time.Time `json:"processed_at"`
}

type LLMRequest struct {
	Model    string        `json:"model"`
	Messages []LLMMessage  `json:"messages"`
	MaxTokens int          `json:"max_tokens,omitempty"`
}

type LLMMessage struct {
    Role    string      `json:"role"`
    Content interface{} `json:"content"`
}

type Content struct {
    Type     string    `json:"type"`
    Text     string    `json:"text,omitempty"`
    ImageUrl *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
    URL string `json:"url"`
}

type LLMResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (ac *AIController) CheckIngredientsRequest(c *gin.Context) (bool,IngredientsRequest){
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return false,IngredientsRequest{}
	}
	log.Printf("Processing image analysis request for user: %v", userID)

	var req IngredientsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid request data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return false,IngredientsRequest{}
	}

	if req.IngredientName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ingredient name is required"})
		return false,IngredientsRequest{}
	}
	log.Printf("Recommending similar ingredients for: %s", req.IngredientName)

	return true,req
}

func (ac *AIController) RecommendSimilarIngredients(c *gin.Context) {
	var req IngredientsRequest
	var checkRequest bool
	checkRequest, req = ac.CheckIngredientsRequest(c)
	if !checkRequest {
		return
	}
	prompt := fmt.Sprintf("可以列举出%s的汉语通用名，或是与其最接近的常见食材, 你需要输出以下格式的内容：{\"name\":\"food_name\"}，否则你只需要输出:{\"name\":\"none\"}.", req.IngredientName)
	outputText, err := ac.callLLMChatAPI(prompt)
	if err != nil {
		log.Printf("Error calling LLM API: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error analyzing image: %v", err)})
		return
	}

	response := ImageAnalysisResponse{
		Text:        outputText,
		ProcessedAt: time.Now(),
	}

	c.JSON(http.StatusOK, response)

}

func (ac *AIController) IntroduceIngredient(c *gin.Context) {
	var req IngredientsRequest
	var checkRequest bool
	checkRequest, req = ac.CheckIngredientsRequest(c)
	if !checkRequest {
		return
	}
	prompt := fmt.Sprintf("你需要对%s进行简单的介绍，介绍不超过200字，应当包括其基本信息、包含的营养、药理学性质以及其他你认为合适的内容", req.IngredientName)
	outputText, err := ac.callLLMChatAPI(prompt)
	if err != nil {
		log.Printf("Error calling LLM API: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error analyzing image: %v", err)})
		return
	}

	response := ImageAnalysisResponse{
		Text:        outputText,
		ProcessedAt: time.Now(),
	}

	c.JSON(http.StatusOK, response)
}

func (ac *AIController) AnalyzeImage(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	log.Printf("Processing image analysis request for user: %v", userID)

	var req ImageAnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid request data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	if req.ImageURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Image URL is required"})
		return
	}
	log.Printf("Analyzing image from URL: %s", req.ImageURL)

	analysisText, err := ac.callLLMImageAPI(req.ImageURL)
	if err != nil {
		log.Printf("Error calling LLM API: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error analyzing image: %v", err)})
		return
	}

	response := ImageAnalysisResponse{
		Text:        analysisText,
		ProcessedAt: time.Now(),
	}

	c.JSON(http.StatusOK, response)
}

func convertURLToBase64(imageURL string) (string, error) {
    // 下载图像
    resp, err := http.Get(imageURL)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    // 读取图像数据
    imageData, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }
    
    // 转换为Base64
    return base64.StdEncoding.EncodeToString(imageData), nil
}

func (ac *AIController) callLLMChatAPI(prompt string) (string, error) {
	apiKey := os.Getenv("LLM_API_KEY")
    if apiKey == "" {
        return "", fmt.Errorf("LLM_API_KEY environment variable not set")
    }

	apiEndpoint := os.Getenv("LLM_API_ENDPOINT")
    if apiEndpoint == "" {
        apiEndpoint = "https://api.moonshot.cn/v1/chat/completions"
    }


	llmReq := LLMRequest{
		Model: "moonshot-v1-8k",
		Messages: []LLMMessage{
			{
				Role: "system",
				Content: "假设你是一个AI助手", // Simple string for system
			},
			{
				Role: "user",
				Content: []Content{ // Array for user with multiple content types
					{
						Type: "text",
						Text: prompt,
					},
				},
			},
		},
		MaxTokens: 300,
	}

	reqBody, err := json.Marshal(llmReq)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	req, err := http.NewRequest("POST", apiEndpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request to LLM API: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var llmResp LLMResponse
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	if llmResp.Error != nil && llmResp.Error.Message != "" {
		return "", fmt.Errorf("LLM API error: %s", llmResp.Error.Message)
	}

	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("no response content received from LLM API")
	}

	return llmResp.Choices[0].Message.Content, nil
}
func (ac *AIController) callLLMImageAPI(imageURL string) (string, error) {
	apiKey := os.Getenv("LLM_API_KEY")
    if apiKey == "" {
        return "", fmt.Errorf("LLM_API_KEY environment variable not set")
    }

	apiEndpoint := os.Getenv("LLM_API_ENDPOINT")
    if apiEndpoint == "" {
        apiEndpoint = "https://api.moonshot.cn/v1/chat/completions"
    }

	imageBase64, err := convertURLToBase64(imageURL)
    if err != nil {
        return "", fmt.Errorf("error converting image to base64: %v", err)
    }

	dataURL := fmt.Sprintf("data:image/jpeg;base64,%s", imageBase64)

	llmReq := LLMRequest{
		Model: "moonshot-v1-8k-vision-preview",
		Messages: []LLMMessage{
			{
				Role: "system",
				Content: "假设你是一个AI助手，用来帮助用户识别图片中的主要食物", // Simple string for system
			},
			{
				Role: "user",
				Content: []Content{ // Array for user with multiple content types
					{
						Type: "text",
						Text: "请尝试描述这个图片，如果你认为图片中的物体包含食物，你需要输出以下格式的内容：{\"name\":\"food_name\"}，否则你只需要输出:{\"name\":\"none\"}.",
					},
					{
						Type: "image_url",
						ImageUrl: &ImageURL{
							URL: dataURL,
						},
					},
				},
			},
		},
		MaxTokens: 300,
	}

	reqBody, err := json.Marshal(llmReq)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	req, err := http.NewRequest("POST", apiEndpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request to LLM API: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var llmResp LLMResponse
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	if llmResp.Error != nil && llmResp.Error.Message != "" {
		return "", fmt.Errorf("LLM API error: %s", llmResp.Error.Message)
	}

	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("no response content received from LLM API")
	}

	return llmResp.Choices[0].Message.Content, nil
}