package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/znsio/specmatic-order-bff-go/internal/models"
)

type BackendService struct {
	BaseURL   string
	AuthToken string
}

func NewBackendService(baseURL string, authToken string) *BackendService {
	return &BackendService{BaseURL: baseURL, AuthToken: authToken}
}

func (s *BackendService) GetAllProducts(productType string, pageSize int) ([]models.Product, error) {
	resp, err := http.Get(s.BaseURL + "/products?type=" + productType)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// If the response is not OK, return the error message from the backend
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("something went wrong, please check if you provided a valid 'type': %w", err)
	}

	var products []models.Product
	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		return nil, err
	}

	// // Send Kafka messages
	err = SendProductMessages(products)
	if err != nil {
		return nil, fmt.Errorf("error sending Kafka messages: %w", err)
	}

	return products, nil
}

func (s *BackendService) CreateProduct(newProduct models.NewProduct) (int, error) {

	apiUrl := s.BaseURL + "/products"

	requestBody, err := json.Marshal(newProduct)
	if err != nil {
		return -1, err
	}

	req, err := http.NewRequest("POST", apiUrl, bytes.NewReader(requestBody))
	if err != nil {
		return -1, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authenticate", s.AuthToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return -1, fmt.Errorf("something went wrong, please try again")
	}

	var responseBody map[string]interface{}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return -1, err
	}
	err = json.Unmarshal(bodyBytes, &responseBody)
	if err != nil {
		return -1, err
	}

	productID, ok := responseBody["id"].(float64)
	if !ok {
		return -1, fmt.Errorf("something went wrong, please try again")
	}

	return int(productID), nil

}

func (s *BackendService) CreateOrder(orderRequest models.OrderRequest) (int, error) {
	apiUrl := s.BaseURL + "/orders" // Assuming this is the endpoint for creating orders

	order := models.Order{
		ProductID: orderRequest.ProductID,
		Count:     orderRequest.Count,
		Status:    "pending",
	}

	requestBody, err := json.Marshal(order)
	if err != nil {
		return -1, fmt.Errorf("error marshalling order: %w", err)
	}

	req, err := http.NewRequest("POST", apiUrl, bytes.NewReader(requestBody))
	if err != nil {
		return -1, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authenticate", s.AuthToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return -1, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil {
			fmt.Println("Response body:", string(bodyBytes))
		}
		return -1, fmt.Errorf("received non-200 response: %s", resp.Status)
	}

	var responseBody map[string]interface{}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return -1, fmt.Errorf("error reading response body: %w", err)
	}

	if len(bodyBytes) == 0 {
		return -1, fmt.Errorf("no order id received in Order API response")
	}

	err = json.Unmarshal(bodyBytes, &responseBody)
	if err != nil {
		return -1, fmt.Errorf("error unmarshalling response body: %w", err)
	}

	orderID, ok := responseBody["id"].(float64)
	if !ok {
		return -1, fmt.Errorf("invalid order id received in response")
	}

	return int(orderID), nil
}
