package service

import (
	"fmt"
	"github.com/chubaofs/chubaofs/proto"
	"github.com/chubaofs/chubaofs/sdk/master"
)

type UserService struct {
	UserApi *master.UserAPI
}

func (u *UserService) Login(args struct {
	UserID    string
	SecretKey string
}) (*proto.UserInfo, error) {

	user, err := u.UserApi.GetUserInfo(args.UserID)
	if err != nil {
		return nil, err
	}

	if user.SecretKey != args.SecretKey {
		return nil, fmt.Errorf("name or password error")
	}

	return user, nil

}

func (u *UserService) Schema() string {
	return `
		schema {
			query: Query
		}
		"the query for user handler"
		type Query {
			login(user_id: String!, access_key: String!): UserInfo
		}

		type UserPolicy {
			own_vols: [String]
		}
	
		type UserInfo {
			userID: String
			accessKey: String
			secretKey: String
			policy: UserPolicy
			userType: Int
			create_time: String
		}
	`
}
