package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"relay/client-backend/internal/domain"

	"github.com/google/uuid"
)

const stripeAPIBaseURL = "https://api.stripe.com/v1"

type PaymentMethodSetupIntent struct {
	ID             string `json:"id"`
	ClientSecret   string `json:"client_secret"`
	StripeCustomer string `json:"stripe_customer_id"`
}

func (s *Services) CreatePaymentMethodSetupIntent(ctx context.Context, actor domain.User, organizationID uuid.UUID) (PaymentMethodSetupIntent, error) {
	client, err := newStripeClientFromEnv()
	if err != nil {
		return PaymentMethodSetupIntent{}, err
	}

	account, err := s.repo.GetBillingAccount(ctx, organizationID)
	if err != nil {
		return PaymentMethodSetupIntent{}, err
	}
	organization, err := s.repo.GetOrganizationByID(ctx, organizationID)
	if err != nil {
		return PaymentMethodSetupIntent{}, err
	}

	stripeCustomerID := ""
	if account.StripeCustomerID != nil {
		stripeCustomerID = strings.TrimSpace(*account.StripeCustomerID)
	}
	if stripeCustomerID == "" {
		customer, err := client.createCustomer(ctx, stripeCustomerParams{
			Email:          account.BillingEmail,
			Name:           organization.Name,
			OrganizationID: organizationID.String(),
		})
		if err != nil {
			return PaymentMethodSetupIntent{}, err
		}
		account, err = s.repo.SetBillingAccountStripeCustomerID(ctx, organizationID, customer.ID)
		if err != nil {
			return PaymentMethodSetupIntent{}, err
		}
		stripeCustomerID = customer.ID
	}

	intent, err := client.createSetupIntent(ctx, stripeCustomerID, organizationID.String())
	if err != nil {
		return PaymentMethodSetupIntent{}, err
	}

	_ = s.audit(ctx, organizationID, &actor.ID, "payment_method.setup_intent_created", "billing_account", &account.ID, map[string]string{
		"stripe_customer_id": stripeCustomerID,
		"setup_intent_id":    intent.ID,
	})

	return PaymentMethodSetupIntent{
		ID:             intent.ID,
		ClientSecret:   intent.ClientSecret,
		StripeCustomer: stripeCustomerID,
	}, nil
}

func (s *Services) ConfirmPaymentMethodSetup(ctx context.Context, actor domain.User, organizationID uuid.UUID, setupIntentID string) (domain.PaymentMethod, error) {
	setupIntentID = strings.TrimSpace(setupIntentID)
	if setupIntentID == "" {
		return domain.PaymentMethod{}, ErrInvalidInput
	}

	client, err := newStripeClientFromEnv()
	if err != nil {
		return domain.PaymentMethod{}, err
	}

	account, err := s.repo.GetBillingAccount(ctx, organizationID)
	if err != nil {
		return domain.PaymentMethod{}, err
	}
	if account.StripeCustomerID == nil || strings.TrimSpace(*account.StripeCustomerID) == "" {
		return domain.PaymentMethod{}, ErrInvalidInput
	}

	intent, err := client.retrieveSetupIntent(ctx, setupIntentID)
	if err != nil {
		return domain.PaymentMethod{}, err
	}
	if intent.Status != "succeeded" || strings.TrimSpace(intent.PaymentMethod) == "" {
		return domain.PaymentMethod{}, ErrInvalidInput
	}
	if intent.Customer != strings.TrimSpace(*account.StripeCustomerID) {
		return domain.PaymentMethod{}, ErrForbidden
	}

	stripeMethod, err := client.retrievePaymentMethod(ctx, intent.PaymentMethod)
	if err != nil {
		return domain.PaymentMethod{}, err
	}
	if stripeMethod.Card == nil {
		return domain.PaymentMethod{}, ErrInvalidInput
	}

	existing, err := s.repo.ListPaymentMethods(ctx, organizationID)
	if err != nil {
		return domain.PaymentMethod{}, err
	}
	method, err := s.repo.CreatePaymentMethod(ctx, domain.PaymentMethod{
		OrganizationID:        organizationID,
		StripePaymentMethodID: stripeMethod.ID,
		Brand:                 stripeMethod.Card.Brand,
		Last4:                 stripeMethod.Card.Last4,
		ExpMonth:              int32(stripeMethod.Card.ExpMonth),
		ExpYear:               int32(stripeMethod.Card.ExpYear),
		IsDefault:             len(existing) == 0,
	})
	if err != nil {
		return domain.PaymentMethod{}, err
	}

	_ = s.audit(ctx, organizationID, &actor.ID, "payment_method.added", "payment_method", &method.ID, map[string]string{
		"brand":                    method.Brand,
		"last4":                    method.Last4,
		"stripe_setup_intent_id":   setupIntentID,
		"stripe_payment_method_id": method.StripePaymentMethodID,
	})

	return method, nil
}

type stripeClient struct {
	secretKey  string
	httpClient *http.Client
}

type stripeCustomerParams struct {
	Email          string
	Name           string
	OrganizationID string
}

type stripeCustomer struct {
	ID string `json:"id"`
}

type stripeCheckoutSessionParams struct {
	CustomerID     string
	OrganizationID string
	OrderID        string
	Name           string
	Description    string
	AmountCents    int64
	Currency       string
	SuccessURL     string
	CancelURL      string
}

type stripeCheckoutSession struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type stripeSetupIntent struct {
	ID            string `json:"id"`
	ClientSecret  string `json:"client_secret"`
	Customer      string `json:"customer"`
	PaymentMethod string `json:"payment_method"`
	Status        string `json:"status"`
}

type stripePaymentMethod struct {
	ID   string            `json:"id"`
	Card *stripeCardRecord `json:"card"`
}

type stripeCardRecord struct {
	Brand    string `json:"brand"`
	Last4    string `json:"last4"`
	ExpMonth int    `json:"exp_month"`
	ExpYear  int    `json:"exp_year"`
}

type stripeErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func newStripeClientFromEnv() (*stripeClient, error) {
	secretKey := strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY"))
	if secretKey == "" {
		return nil, ErrStripeNotConfigured
	}
	return &stripeClient{
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}, nil
}

func (c *stripeClient) createCustomer(ctx context.Context, params stripeCustomerParams) (stripeCustomer, error) {
	values := url.Values{}
	values.Set("email", params.Email)
	values.Set("name", params.Name)
	values.Set("metadata[organization_id]", params.OrganizationID)

	var customer stripeCustomer
	err := c.postForm(ctx, "/customers", values, "customer-"+params.OrganizationID, &customer)
	return customer, err
}

func (c *stripeClient) createSetupIntent(ctx context.Context, customerID, organizationID string) (stripeSetupIntent, error) {
	values := url.Values{}
	values.Set("customer", customerID)
	values.Set("usage", "off_session")
	values.Add("payment_method_types[]", "card")
	values.Set("metadata[organization_id]", organizationID)

	var intent stripeSetupIntent
	err := c.postForm(ctx, "/setup_intents", values, "setup-intent-"+organizationID+"-"+uuid.NewString(), &intent)
	return intent, err
}

func (c *stripeClient) createCheckoutSession(ctx context.Context, params stripeCheckoutSessionParams) (stripeCheckoutSession, error) {
	values := url.Values{}
	values.Set("mode", "payment")
	values.Set("customer", params.CustomerID)
	values.Set("success_url", params.SuccessURL)
	values.Set("cancel_url", params.CancelURL)
	values.Set("line_items[0][quantity]", "1")
	values.Set("line_items[0][price_data][currency]", strings.ToLower(params.Currency))
	values.Set("line_items[0][price_data][unit_amount]", fmt.Sprintf("%d", params.AmountCents))
	values.Set("line_items[0][price_data][product_data][name]", params.Name)
	if strings.TrimSpace(params.Description) != "" {
		values.Set("line_items[0][price_data][product_data][description]", params.Description)
	}
	values.Set("metadata[organization_id]", params.OrganizationID)
	values.Set("metadata[order_id]", params.OrderID)
	values.Set("payment_intent_data[metadata][organization_id]", params.OrganizationID)
	values.Set("payment_intent_data[metadata][order_id]", params.OrderID)
	values.Set("payment_intent_data[setup_future_usage]", "off_session")

	var session stripeCheckoutSession
	err := c.postForm(ctx, "/checkout/sessions", values, "checkout-"+params.OrderID, &session)
	return session, err
}

func (c *stripeClient) retrieveSetupIntent(ctx context.Context, setupIntentID string) (stripeSetupIntent, error) {
	var intent stripeSetupIntent
	err := c.get(ctx, "/setup_intents/"+url.PathEscape(setupIntentID), &intent)
	return intent, err
}

func (c *stripeClient) retrievePaymentMethod(ctx context.Context, paymentMethodID string) (stripePaymentMethod, error) {
	var method stripePaymentMethod
	err := c.get(ctx, "/payment_methods/"+url.PathEscape(paymentMethodID), &method)
	return method, err
}

func (c *stripeClient) postForm(ctx context.Context, path string, values url.Values, idempotencyKey string, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, stripeAPIBaseURL+path, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	return c.do(request, out)
}

func (c *stripeClient) get(ctx context.Context, path string, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, stripeAPIBaseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(request, out)
}

func (c *stripeClient) do(request *http.Request, out any) error {
	request.SetBasicAuth(c.secretKey, "")
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrStripeRequestFailed, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		var stripeErr stripeErrorResponse
		if err := json.Unmarshal(body, &stripeErr); err == nil && stripeErr.Error.Message != "" {
			message = stripeErr.Error.Message
		}
		if message == "" {
			message = response.Status
		}
		return fmt.Errorf("%w: %s", ErrStripeRequestFailed, message)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(out); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}
