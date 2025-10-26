package auth_test

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	sut "github.com/Builder-Lawyers/builder-backend/internal/application/commands/auth"
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

//var (
//	authSetupOnce sync.Once
//	authDeps      *awsC
//)
//
//func setupAuthDepsOnce(t *testing.T) {
//	authSetupOnce.Do(func() {
//		// build cognitoClient etc once
//		authDeps = newAuthDeps()
//	})
//}

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
	userID, err := createUser(ctx, email)
	require.NoError(t, err)
	req, err := getRequest(ctx, email)
	require.NoError(t, err)
	fmt.Println(req)
	SUT := sut.NewAuth(db.NewUoWFactory(testinfra.Pool), auth.NewOIDCConfig(), testinfra.AwsCfg)

	sessionID, err := SUT.CreateSession(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, sessionID)

	var foundUserID uuid.UUID
	var rt sql.NullString
	var issuedAt time.Time
	row := testinfra.Pool.QueryRow(ctx, `SELECT user_id, refresh_token, issued_at FROM builder.sessions WHERE id = $1`, sessionID)
	err = row.Scan(&foundUserID, &rt, &issuedAt)
	require.NoError(t, err)
	require.Equal(t, *userID, foundUserID, "session user mismatch")
}

func getRequest(ctx context.Context, email string) (dto.CreateSession, error) {
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
		return dto.CreateSession{}, fmt.Errorf("admin initiate auth: %v", err)
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
	}, nil
}

func createUser(ctx context.Context, email string) (*uuid.UUID, error) {
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
		return nil, fmt.Errorf("admin create: %w", err)
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
		return nil, fmt.Errorf("admin set userPassword: %w", err)
	}

	_, err = testinfra.Pool.Exec(ctx, `INSERT INTO builder.users (id, email) VALUES ($1,$2)`, userID, email)
	if err != nil {
		return nil, fmt.Errorf("user insert: %w", err)
	}

	return &userID, nil
}

func clearTestState(t testing.TB, email string) {
	ctx := context.Background()

	_, _ = cognitoClient.AdminDisableUser(ctx, &cognitoidentityprovider.AdminDisableUserInput{
		UserPoolId: aws.String(cognitoPoolID),
		Username:   aws.String(email),
	})

	_, _ = cognitoClient.AdminDeleteUser(ctx, &cognitoidentityprovider.AdminDeleteUserInput{
		UserPoolId: aws.String(cognitoPoolID),
		Username:   aws.String(email),
	})

	_, _ = testinfra.Pool.Exec(ctx, `DELETE FROM builder.users WHERE email=$1`, email)
}
