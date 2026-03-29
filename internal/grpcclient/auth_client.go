package grpcclient

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authpb "github.com/luckysxx/common/proto/auth"
)

func NewAuthClient(userPlatformAddr string) (authpb.AuthServiceClient, error) {
	conn, err := grpc.NewClient(userPlatformAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return authpb.NewAuthServiceClient(conn), nil
}
