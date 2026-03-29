package grpcclient

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	userpb "github.com/luckysxx/common/proto/user"
)

func NewUserClient(userPlatformAddr string) (userpb.UserServiceClient, error) {
	conn, err := grpc.NewClient(userPlatformAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return userpb.NewUserServiceClient(conn), nil
}
