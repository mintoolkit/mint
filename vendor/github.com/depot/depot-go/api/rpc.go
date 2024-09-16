package api

import (
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/depot/depot-go/proto/depot/cli/v1/cliv1connect"
)

func NewBuildClient() cliv1connect.BuildServiceClient {
	baseURL := os.Getenv("DEPOT_API_URL")
	if baseURL == "" {
		baseURL = "https://api.depot.dev"
	}
	return cliv1connect.NewBuildServiceClient(http.DefaultClient, baseURL, WithUserAgent())
}

func WithAuthentication[T any](req *connect.Request[T], token string) *connect.Request[T] {
	req.Header().Add("Authorization", "Bearer "+token)
	return req
}
