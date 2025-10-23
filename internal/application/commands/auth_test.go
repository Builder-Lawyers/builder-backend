package commands

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var pool *pgxpool.Pool
var cognitoClient *cognitoidentityprovider.Client
var cognitoPoolID string
var cognitoClientID string
var awsCfg aws.Config

func TestMain(m *testing.M) {

	ctx := context.Background()

	pgReq := testcontainers.ContainerRequest{
		Image:        "postgres:17.2-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_PASSWORD": "password",
			"POSTGRES_USER":     "postgres",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections"),
	}
	pgC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: pgReq,
		Started:          true,
	})
	if err != nil {
		log.Panicf("start postgres: %v", err)
	}
	defer pgC.Terminate(ctx)

	pgHostPort, err := pgC.Endpoint(ctx, "")
	if err != nil {
		log.Panicf("postgres endpoint: %v", err)
	}
	pgDSN := fmt.Sprintf("postgres://postgres:password@%s/testdb?sslmode=disable", pgHostPort)
	time.Sleep(1 * time.Second)

	pool, err = pgxpool.New(ctx, pgDSN)
	if err != nil {
		log.Panicf("pgxpool connect: %v", err)
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS builder;
		CREATE TABLE IF NOT EXISTS builder.users (
		  id UUID PRIMARY KEY,
		  email TEXT UNIQUE NOT NULL
		);
		CREATE TABLE IF NOT EXISTS builder.sessions (
		  id UUID PRIMARY KEY,
		  user_id UUID NOT NULL REFERENCES builder.users(id),
		  refresh_token TEXT,
		  issued_at TIMESTAMP WITH TIME ZONE
		);
	`)
	if err != nil {
		log.Panicf("create tables: %v", err)
	}

	awsCfg, err = awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Panic("can't load aws config", err)
	}
	cognitoPoolID = os.Getenv("COGNITO_POOL_ID")
	cognitoClientID = os.Getenv("COGNITO_CLIENT_ID")
	cognitoClient = cognitoidentityprovider.NewFromConfig(awsCfg, func(o *cognitoidentityprovider.Options) {
		o.Region = awsCfg.Region
	})

	code := m.Run()

	pool.Close()
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
	SUT := NewAuth(db.NewUoWFactory(pool), auth.NewOIDCConfig(), awsCfg)

	sessionID, err := SUT.CreateSession(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, sessionID)

	var foundUserID uuid.UUID
	var rt sql.NullString
	var issuedAt time.Time
	row := pool.QueryRow(ctx, `SELECT user_id, refresh_token, issued_at FROM builder.sessions WHERE id = $1`, sessionID)
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
		fmt.Printf("AttName:%v VAL: %v\n", *attribute.Name, *attribute.Value)
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

	_, err = pool.Exec(ctx, `INSERT INTO builder.users (id, email) VALUES ($1,$2)`, userID, email)
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

	_, _ = pool.Exec(ctx, `DELETE FROM builder.users WHERE email=$1`, email)
}
