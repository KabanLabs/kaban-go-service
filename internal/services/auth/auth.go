package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/VACdotCS/kaban-go-service/internal/config"
	"github.com/VACdotCS/kaban-go-service/internal/metrics"
	ssov1 "github.com/VACdotCS/protos/gen/go/sso"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	api ssov1.AuthClient
	log *slog.Logger
}

func New(ctx context.Context, cfg *config.AuthConfig, logger *slog.Logger) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	conn, err := grpc.DialContext(ctx, cfg.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to auth service: %w", err)
	}

	return &Client{
		api: ssov1.NewAuthClient(conn),
		log: logger,
	}, nil
}

func (c *Client) ValidateToken(ctx context.Context, token string) (bool, error) {
	// Create context with timeout if not provided
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	timer := prometheus.NewTimer(metrics.SSOAuthDuration)
	resp, err := c.api.ValidateToken(ctx, &ssov1.ValidateTokenRequest{
		AccessToken: token,
	})
	timer.ObserveDuration()

	if err != nil {
		c.log.Error("Failed to validate token via gRPC", "error", err)
		return false, err
	}

	return resp.Valid, nil
}
