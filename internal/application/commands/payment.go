package commands

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db/repo"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/mail"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/subscription"
	"github.com/stripe/stripe-go/v82/webhook"
)

type Payment struct {
	uowFactory *dbs.UOWFactory
	eventRepo  *repo.EventRepo
	cfg        *PaymentConfig
}

type PaymentConfig struct {
	apiKey     string
	webhookKey string
	returnUrl  string
}

func NewPaymentConfig() *PaymentConfig {
	return &PaymentConfig{
		apiKey:     os.Getenv("STRIPE_KEY"),
		webhookKey: os.Getenv("STRIPE_WEBHOOK"),
		returnUrl:  os.Getenv("STRIPE_RETURN_URL"),
	}
}

func NewPayment(uowFactory *dbs.UOWFactory, cfg *PaymentConfig) *Payment {
	stripe.Key = cfg.apiKey
	stripe.SetHTTPClient(&http.Client{Timeout: 10 * time.Second})
	return &Payment{
		uowFactory: uowFactory,
		cfg:        cfg,
	}
}

func (c *Payment) CreatePayment(req *dto.CreatePaymentRequest, identity *auth.Identity) (string, error) {

	slog.Info("START CHECKOUT")
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return "", fmt.Errorf("error starting tx, %v", err)
	}
	var existingSubID string
	err = tx.QueryRow(context.Background(), "SELECT subscription_id FROM builder.sites WHERE id = $1", req.SiteID).Scan(&existingSubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Info("There's no subscription for site yet", "siteID", req.SiteID)
		} else {
			return "", fmt.Errorf("error retrieving subscription, %v", err)
		}
	}

	var stripePlanID string
	err = tx.QueryRow(context.Background(), "SELECT stripe_id FROM builder.payment_plans WHERE id = $1",
		req.PlanID).Scan(&stripePlanID)
	if err != nil {
		return "", fmt.Errorf("error retrieving stripe price, %v", err)
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return "", fmt.Errorf("error commiting tx, %v", err)
	}

	// TODO: if it is a simple plan, create a subscription with free trial and no payment method details requested
	// otherwise create a checkout
	// Send email that tells user that another mail will be sent N days before trial end for them to add payment method

	var subParams = &stripe.CheckoutSessionSubscriptionDataParams{
		TrialSettings: &stripe.CheckoutSessionSubscriptionDataTrialSettingsParams{
			EndBehavior: &stripe.CheckoutSessionSubscriptionDataTrialSettingsEndBehaviorParams{
				MissingPaymentMethod: stripe.String("cancel"),
			},
		},
	}
	if stripePlanID == "1" {
		subParams.TrialEnd = stripe.Int64(60)
	}

	params := &stripe.CheckoutSessionParams{
		UIMode:    stripe.String("embedded"),
		ReturnURL: stripe.String(c.cfg.returnUrl + "/complete?session_id={CHECKOUT_SESSION_ID}"),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(stripePlanID),
				Quantity: stripe.Int64(1),
			},
		},
		Mode:             stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SubscriptionData: subParams,
	}

	slog.Info("Creating a checkout session")
	s, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("error creating session: %v", err)
	}
	slog.Info("After create session")

	return s.ClientSecret, nil
}

func (c *Payment) GetPaymentInfo(sessionID string) (*dto.PaymentStatusResponse, error) {
	//params := &stripe.CheckoutSessionParams{}
	//params.AddExpand("payment_intent")
	s, err := session.Get(sessionID, &stripe.CheckoutSessionParams{})
	if err != nil {
		return nil, fmt.Errorf("error getting session info, %v", err)
	}

	// customer.id

	sub, err := subscription.Get(s.Subscription.ID, &stripe.SubscriptionParams{})
	if err != nil {
		return nil, fmt.Errorf("error getting subscription %v", err)
	}
	fmt.Println(*sub)

	return &dto.PaymentStatusResponse{
		Status:        string(s.Status),
		PaymentStatus: string(s.PaymentStatus),
		//PaymentIntentID:     s.PaymentIntent.ID,
		//PaymentIntentStatus: string(s.PaymentIntent.Status),
	}, nil
}

func (c *Payment) Webhook(req []byte, stripeHeader string) error {
	event, err := webhook.ConstructEvent(req, stripeHeader, c.cfg.webhookKey)
	if err != nil {
		return fmt.Errorf("error creating event, %v", err)
	}

	fmt.Println(event)

	switch event.Type {
	case "customer.subscription.trial_will_end":
		var subscription stripe.Subscription
		err = json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			return fmt.Errorf("error parsing subscription, %v", err)
		}

		fmt.Printf("Trial will end for subscription %s (customer %s)", subscription.ID, subscription.Customer.ID)

		uow := c.uowFactory.GetUoW()
		tx, err := uow.Begin()
		if err != nil {
			return fmt.Errorf("error starting tx, %v", err)
		}
		var userID string
		err = tx.QueryRow(context.Background(), "SELECT id FROM builder.users WHERE stripe_id = $1",
			subscription.Customer.ID).Scan(&userID)
		if err != nil {
			return fmt.Errorf("err finding user, %v", err)
		}

		session, err := session.New(&stripe.CheckoutSessionParams{
			Customer:           stripe.String(subscription.Customer.ID),
			UIMode:             stripe.String("embedded"),
			Mode:               stripe.String("setup"),
			PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
			//SuccessURL: stripe.String("https://localhost:8080/success"),
			//CancelURL:  stripe.String("https://localhost:8080/cancel"),
		})

		mailData := mail.FreeTrialEndsData{
			// TODO: subscription.TrialEnd if in epoch second,
			DaysUntilEnd: 7,
			PaymentURL:   session.URL,
		}

		sendMailEvent := events.SendMail{
			UserID:  userID,
			Subject: string(mailData.GetMailType()),
			Data:    mailData,
		}
		err = c.eventRepo.InsertEvent(tx, sendMailEvent)
		if err != nil {
			return fmt.Errorf("failed to send an email, %v", err)
		}

		slog.Info("Event sendMail created", "subID", subscription.ID)

	default:
		return fmt.Errorf("Unhandled event type: %s\n", event.Type)
	}

	return nil
}
