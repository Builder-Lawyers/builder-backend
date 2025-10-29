package auth_test

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	sut "github.com/Builder-Lawyers/builder-backend/internal/application/commands/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/testinfra"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var cognitoClient *cognitoidentityprovider.Client
var cognitoPoolID string
var cognitoClientID string

func TestMain(m *testing.M) {
	awsCfg := testinfra.AwsCfg

	cognitoPoolID = os.Getenv("COGNITO_POOL_ID")
	cognitoClientID = os.Getenv("COGNITO_CLIENT_ID")
	cognitoClient = cognitoidentityprovider.NewFromConfig(awsCfg, func(o *cognitoidentityprovider.Options) {
		o.Region = "us-east-1"
	})

	code := m.Run()

	slog.Info("Tests are completed")

	os.Exit(code)
}

func Test_CreateSession_Given_Existing_User_When_Called_Then_Create_Session_And_Return_ID(t *testing.T) {
	ctx := context.Background()
	email := "example10@gmail.com"
	defer clearTestState(t, email)
	userID := createUser(ctx, email)
	req := getRequest(ctx, email)
	SUT := sut.NewAuth(db.NewUoWFactory(testinfra.Pool), auth.NewOIDCConfig(), cognitoClient)

	sessionID, err := SUT.CreateSession(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, sessionID)

	var foundUserID uuid.UUID
	var rt sql.NullString
	var expiresAt time.Time
	row := testinfra.Pool.QueryRow(ctx, `SELECT user_id, refresh_token, expires_at FROM builder.sessions WHERE id = $1`, sessionID)
	err = row.Scan(&foundUserID, &rt, &expiresAt)
	require.NoError(t, err)
	require.Equal(t, userID, foundUserID, "session user mismatch")
	require.Greater(t, expiresAt, time.Now())
}

func Test_GetSession_Given_Valid_Session_Cookie_In_Request_When_Called_Then_Get_User_Info(t *testing.T) {
	ctx := context.Background()
	email := "example10@gmail.com"
	defer clearTestState(t, email)
	userID := createUser(ctx, email)
	siteID := createSite(ctx, userID)
	req := getRequest(ctx, email)

	SUT := sut.NewAuth(db.NewUoWFactory(testinfra.Pool), auth.NewOIDCConfig(), cognitoClient)
	sessionID, err := SUT.CreateSession(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, sessionID)

	userInfo, err := SUT.GetSession(ctx, uuid.MustParse(sessionID))
	require.NoError(t, err)
	require.Equal(t, userID, userInfo.UserID)
	require.Equal(t, email, userInfo.Email)
	require.Equal(t, siteID, userInfo.UserSite.SiteID)
}

func getRequest(ctx context.Context, email string) dto.CreateSession {
	authOut, err := cognitoClient.AdminInitiateAuth(ctx, &cognitoidentityprovider.AdminInitiateAuthInput{
		UserPoolId: &cognitoPoolID,
		ClientId:   &cognitoClientID,
		AuthFlow:   "ADMIN_USER_PASSWORD_AUTH",
		AuthParameters: map[string]string{
			"USERNAME": email,
			"PASSWORD": "Passw0rd!",
		},
	})
	if err != nil {
		log.Panicf("admin initiate auth: %v", err)
	}

	idToken := *authOut.AuthenticationResult.IdToken
	accessToken := *authOut.AuthenticationResult.AccessToken
	refreshToken := ""
	if authOut.AuthenticationResult.RefreshToken != nil {
		refreshToken = *authOut.AuthenticationResult.RefreshToken
	}

	return dto.CreateSession{
		AccessToken:  accessToken,
		IdToken:      idToken,
		RefreshToken: refreshToken,
	}
}

func createUser(ctx context.Context, email string) uuid.UUID {
	// Create user and set permanent password
	var userID uuid.UUID
	adminCreateResponse, err := cognitoClient.AdminCreateUser(ctx, &cognitoidentityprovider.AdminCreateUserInput{
		UserPoolId: &cognitoPoolID,
		Username:   &email,
		UserAttributes: []types.AttributeType{
			{Name: aws.String("email"), Value: aws.String(email)},
			{Name: aws.String("email_verified"), Value: aws.String("true")},
		},
	})
	if err != nil {
		log.Panicf("admin create: %v", err)
	}
	for _, attribute := range adminCreateResponse.User.Attributes {
		if *attribute.Name == "sub" {
			userID = uuid.MustParse(*attribute.Value)
		}
	}

	_, err = cognitoClient.AdminSetUserPassword(ctx, &cognitoidentityprovider.AdminSetUserPasswordInput{
		UserPoolId: &cognitoPoolID,
		Username:   &email,
		Password:   aws.String("Passw0rd!"),
		Permanent:  true,
	})
	if err != nil {
		log.Panicf("admin set userPassword: %v", err)
	}

	_, err = testinfra.Pool.Exec(ctx, `INSERT INTO builder.users (id, email) VALUES ($1,$2)`, userID, email)
	if err != nil {
		log.Panicf("user insert: %v", err)
	}

	return userID
}

func createSite(ctx context.Context, userID uuid.UUID) uint64 {
	var siteID uint64
	err := testinfra.Pool.QueryRow(ctx, "INSERT INTO builder.sites(template_id, creator_id, plan_id, status, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		1, userID, 1, consts.SiteStatusCreated, time.Now()).Scan(&siteID)
	if err != nil {
		log.Panicf("err creating site, %v", err)
	}
	return siteID
}

func clearTestState(t testing.TB, email string) {
	ctx := context.Background()
	slog.Info("clearing after auth test")

	_, err := cognitoClient.AdminDisableUser(ctx, &cognitoidentityprovider.AdminDisableUserInput{
		UserPoolId: aws.String(cognitoPoolID),
		Username:   aws.String(email),
	})
	if err != nil {
		log.Panicf("err disabling cognito user, %v", err)
	}

	_, err = cognitoClient.AdminDeleteUser(ctx, &cognitoidentityprovider.AdminDeleteUserInput{
		UserPoolId: aws.String(cognitoPoolID),
		Username:   aws.String(email),
	})
	if err != nil {
		log.Panicf("err deleting cognito user, %v", err)
	}

	_, err = testinfra.Pool.Exec(ctx, `DELETE FROM builder.sessions`)
	if err != nil {
		log.Panicf("err clearing sessions, %v", err)
	}

	_, err = testinfra.Pool.Exec(ctx, `DELETE FROM builder.users WHERE email=$1`, email)
	if err != nil {
		log.Panicf("err clearing user, %v", err)
	}
}
