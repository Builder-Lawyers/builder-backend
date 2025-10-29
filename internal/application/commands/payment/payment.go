package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
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
	cfg        PaymentConfig
}

type PaymentConfig struct {
	apiKey     string
	webhookKey string
	returnUrl  string
}

func NewPaymentConfig() PaymentConfig {
	return PaymentConfig{
		apiKey:     os.Getenv("STRIPE_KEY"),
		webhookKey: os.Getenv("STRIPE_WEBHOOK"),
		returnUrl:  os.Getenv("STRIPE_RETURN_URL"),
	}
}

func NewPayment(uowFactory *dbs.UOWFactory, cfg PaymentConfig) *Payment {
	stripe.Key = cfg.apiKey
	stripe.SetHTTPClient(&http.Client{Timeout: 10 * time.Second})
	return &Payment{
		uowFactory: uowFactory,
		cfg:        cfg,
	}
}

func (c *Payment) CreatePayment(ctx context.Context, req *dto.CreatePaymentRequest, identity *auth.Identity) (string, error) {

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return "", err
	}
	// TODO: improve uow api to be able to only rollback at the end of a function, not commit
	defer uow.Finalize(&err)
	var existingSubID sql.NullString
	err = tx.QueryRow(ctx, "SELECT subscription_id FROM builder.sites WHERE id = $1", req.SiteID).Scan(&existingSubID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Info("There's no subscription for site yet", "siteID", req.SiteID)
		} else {
			return "", fmt.Errorf("error retrieving subscription, %v", err)
		}
	}

	if existingSubID.Valid {
		return "", fmt.Errorf("subscription is already created for this site")
	}

	var stripePlanID string
	err = tx.QueryRow(ctx, "SELECT stripe_id FROM builder.payment_plans WHERE id = $1",
		req.PlanID).Scan(&stripePlanID)
	if err != nil {
		return "", fmt.Errorf("error retrieving stripe price, %v", err)
	}

	if req.PlanID == 1 {
		s, err := subscription.New(&stripe.SubscriptionParams{
			Customer: stripe.String("cus_SzleNRbLmsHvcs"),
			Items: []*stripe.SubscriptionItemsParams{
				{
					Price:    stripe.String(stripePlanID),
					Quantity: stripe.Int64(1),
				},
			},
			TrialPeriodDays: stripe.Int64(2), // TODO: make configurable, 2 months by default
		})
		if err != nil {
			return "", fmt.Errorf("error creating sub, %v", err)
		}

		_, err = tx.Exec(ctx, "UPDATE builder.sites SET subscription_id = $1 WHERE id = $2", s.ID, req.SiteID)
		if err != nil {
			return "", fmt.Errorf("err updating site subscription, %v", err)
		}

		return s.ID, nil
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

func (c *Payment) GetPaymentInfo(ctx context.Context, sessionID string) (*dto.PaymentStatusResponse, error) {
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
		//PaymentIntentStatus: string(s.PaymentIntent.SiteStatus),
	}, nil
}

func (c *Payment) ListPaymentPlans(ctx context.Context) (*dto.PaymentPlanList, error) {
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}
	defer uow.Finalize(&err)

	paymentPlans := make([]dto.PaymentPlan, 0, 2)

	rows, err := tx.Query(ctx, "SELECT id, description, price FROM builder.payment_plans")
	if err != nil {
		return nil, fmt.Errorf("err getting plans %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var plan dto.PaymentPlan
		if err = rows.Scan(&plan.Id, &plan.Description, &plan.Price); err != nil {
			return nil, err
		}
		paymentPlans = append(paymentPlans, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &paymentPlans, nil
}

func (c *Payment) Webhook(ctx context.Context, req []byte, stripeHeader string) error {
	event, err := webhook.ConstructEvent(req, stripeHeader, c.cfg.webhookKey)
	if err != nil {
		return fmt.Errorf("error creating event, %v", err)
	}

	slog.Info("Handling event", "type", event.Type)

	switch event.Type {

	case "customer.subscription.trial_will_end":
		return c.handleTrialEnds(ctx, event)

	case "invoice.payment_failed":
		return c.handlePaymentFailed(ctx, event)

	default:
		return fmt.Errorf("Unhandled event type: %s\n", event.Type)
	}

}

func (c *Payment) handleTrialEnds(ctx context.Context, event stripe.Event) error {
	var subscription stripe.Subscription
	err := json.Unmarshal(event.Data.Raw, &subscription)
	if err != nil {
		return fmt.Errorf("error parsing subscription, %v", err)
	}

	slog.Info("Trial will end for subscription", "sub", subscription.ID, "customer", subscription.Customer.ID)

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return err
	}
	var userID string
	var firstName string
	var secondName string
	err = tx.QueryRow(ctx, "SELECT id, first_name, second_name FROM builder.users WHERE stripe_id = $1",
		subscription.Customer.ID).Scan(&userID, &firstName, &secondName)
	if err != nil {
		return fmt.Errorf("err finding user, %v", err)
	}

	session, err := session.New(&stripe.CheckoutSessionParams{
		Customer:           stripe.String(subscription.Customer.ID),
		Mode:               stripe.String("setup"),
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		SuccessURL:         stripe.String("https://localhost:3000/success"),
		CancelURL:          stripe.String("https://localhost:3000/cancel"),
	})

	secondsUntilEnd := subscription.TrialEnd - time.Now().Unix()
	daysUntilEnd := int(math.Ceil(float64(secondsUntilEnd) / 86400.0))

	mailData := mail.FreeTrialEndsData{
		DaysUntilEnd:       daysUntilEnd,
		PaymentURL:         session.URL,
		Year:               strconv.Itoa(time.Now().Year()),
		CustomerFirstName:  firstName,
		CustomerSecondName: secondName,
	}

	sendMailEvent := events.SendMail{
		UserID:  userID,
		Subject: mailData.GetSubject(),
		Data:    mailData,
	}
	eventRepo := repo.NewEventRepo(tx)
	err = eventRepo.InsertEvent(ctx, sendMailEvent)
	if err != nil {
		return fmt.Errorf("failed to send an email, %v", err)
	}

	slog.Info("Event sendMail created", "subID", subscription.ID)

	err = uow.Commit()
	if err != nil {
		return fmt.Errorf("error commiting, %v", err)
	}

	return nil
}

func (c *Payment) handlePaymentFailed(ctx context.Context, event stripe.Event) error {

	// TODO: before deactivating site totally, send an email warning a user that site is about to be deactivated
	// if user doesn't retry payment (if it was a failure in payment)
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return fmt.Errorf("error parsing event, %v", err)
	}
	var subID string
	if invoice.Parent != nil &&
		invoice.Parent.Type == stripe.InvoiceParentTypeSubscriptionDetails &&
		invoice.Parent.SubscriptionDetails != nil &&
		invoice.Parent.SubscriptionDetails.Subscription != nil {
		subID = invoice.Parent.SubscriptionDetails.Subscription.ID
	}

	// TODO: if invoice.Parent has no SubscriptionID field
	//params := &stripe.InvoiceParams{}
	//params.AddExpand("parent.subscription_details.subscription")
	//
	//got, err := invoice.Get(invoice.ID, params)
	//if err != nil {
	//	return fmt.Errorf("error getting invoice, %v", err)
	//}
	//
	//sub := got.Parent.SubscriptionDetails.Subscription
	sub, err := subscription.Get(subID, &stripe.SubscriptionParams{})
	if err != nil {
		return fmt.Errorf("error getting subscription, %v", err)
	}

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return err
	}

	var siteID uint64
	var siteStatus consts.SiteStatus
	err = tx.QueryRow(ctx, "SELECT id, status FROM builder.sites WHERE subscription_id = $1",
		sub.ID).Scan(&siteID, &siteStatus)
	if err != nil {
		return fmt.Errorf("error getting site for subscription, %v", err)
	}

	if siteStatus == consts.SiteStatusCreated {
		deactivateSite := events.DeactivateSite{
			SiteID: siteID,
			Reason: "Payment for subscription wasn't successful",
		}

		eventRepo := repo.NewEventRepo(tx)
		err = eventRepo.InsertEvent(ctx, deactivateSite)
		if err != nil {
			return fmt.Errorf("error creating event, %v", err)
		}

	} else {
		slog.Info("Site is not provisioned yet", "siteID", siteID)
	}

	err = uow.Commit()
	if err != nil {
		return fmt.Errorf("error commiting tx, %v", err)
	}

	return nil
}
