package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	test "github.com/BOBAHDEP/grpc-test/example/service"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/fullstorydev/grpcui/standalone"
	_ "github.com/lib/pq"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const defaultPort = "8080"

var db *sql.DB

const addUser = `
INSERT INTO users (
  age,
  name,
  user_type
) VALUES (
  $1, 
  $2,
  $3
)
RETURNING id, name, age, created_at, user_type `

const updateUser = `
UPDATE users SET
  age = $1,
  name = $2,
  user_type = $3,
  updated_at = CURRENT_TIMESTAMP
WHERE id = $4
RETURNING id, name, age, created_at, updated_at, user_type `

const deleteUser = `
DELETE FROM users
WHERE id = $1`

const deleteItems = `
DELETE FROM items
WHERE user_id = $1`

const getUser = `
SELECT id, name, age, created_at, updated_at, user_type FROM users
WHERE id = $1`

const listUsers = `
SELECT id, name, age, created_at, updated_at, user_type FROM users
ORDER BY id LIMIT $1 OFFSET $2`

const addItem = `
INSERT INTO items (
  name,
  user_id
) VALUES (
  $1, 
  $2
)
RETURNING id, name, user_id, created_at `

const updateItem = `
UPDATE items SET
  name = $1,
  updated_at = CURRENT_TIMESTAMP
WHERE id = $2
RETURNING id, name, user_id, created_at, updated_at `

const listItems = `
SELECT id, name, user_id, created_at, updated_at FROM items
WHERE user_id = $1`

type server struct {
	test.ServiceExampleServiceServer
}

type UserPG struct {
	Id       string
	Name     string
	Age      int32
	UserType string
	//	Items     []*Item
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ItemPG struct {
	Id        string
	Name      string
	UserId    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func typePostgresToProto(pgType string) (test.UserType, error) {
	switch pgType {
	case "UserType_INVALID_USER_TYPE":
		return test.UserType_INVALID_USER_TYPE, nil
	case "UserType_EMPLOYEE_USER_TYPE":
		return test.UserType_EMPLOYEE_USER_TYPE, nil
	case "UserType_CUSTOMER_USER_TYPE":
		return test.UserType_CUSTOMER_USER_TYPE, nil
	default:
		return test.UserType_INVALID_USER_TYPE, nil
	}
}

func typeProtoToPostgres(usertype test.UserType) (string, error) {
	switch usertype {
	case test.UserType_INVALID_USER_TYPE:
		return "UserType_INVALID_USER_TYPE", nil
	case test.UserType_EMPLOYEE_USER_TYPE:
		return "UserType_EMPLOYEE_USER_TYPE", nil
	case test.UserType_CUSTOMER_USER_TYPE:
		return "UserType_CUSTOMER_USER_TYPE", nil
	default:
		return "", nil
	}
}

func userPostgresToProto(pgUser UserPG) (*test.User, error) {
	protoRole, err := typePostgresToProto(pgUser.UserType)
	items, err := ListItems(pgUser.Id)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &test.User{
		CreatedAt: timestamppb.New(pgUser.CreatedAt),
		UpdatedAt: timestamppb.New(pgUser.UpdatedAt),
		Id:        pgUser.Id,
		Name:      pgUser.Name,
		Age:       pgUser.Age,
		UserType:  protoRole,
		Items:     items,
	}, nil
}

func itemPostgresToProto(pgItem ItemPG) (*test.Item, error) {
	return &test.Item{
		CreatedAt: timestamppb.New(pgItem.CreatedAt),
		UpdatedAt: timestamppb.New(pgItem.UpdatedAt),
		Id:        pgItem.Id,
		Name:      pgItem.Name,
		UserId:    pgItem.UserId,
	}, nil
}

func (*server) CreateUser(ctx context.Context, req *test.CreateUserRequest) (*test.User, error) {
	fmt.Println("Create User ", db)
	typeProto, err := typeProtoToPostgres(req.UserType)
	if err != nil {
		return nil, err
	}

	for _, element := range req.Items {
		_, err := CreateItem(ctx, element)
		if err != nil {
			return nil, err
		}
	}

	row := db.QueryRowContext(ctx, addUser, req.Age, req.Name, typeProto)
	fmt.Println("Create row ", row)
	var i UserPG
	err = row.Scan(
		&i.Id,
		&i.Name,
		&i.Age,
		&i.CreatedAt,
		&i.UserType,
	)
	if err != nil {
		return nil, err
	}
	return userPostgresToProto(i)
}

func (*server) UpdateUser(ctx context.Context, req *test.UpdateUserRequest) (*test.User, error) {
	fmt.Println("Update user")
	typeProto, err := typeProtoToPostgres(req.UserType)
	if err != nil {
		return nil, err
	}

	for _, element := range req.Items {
		_, err := UpdateItem(ctx, element)
		if err != nil {
			return nil, err
		}
	}

	row := db.QueryRowContext(ctx, updateUser, req.Age, req.Name, typeProto, req.Id)
	var userPG UserPG
	err = row.Scan(
		&userPG.Id,
		&userPG.Name,
		&userPG.Age,
		&userPG.CreatedAt,
		&userPG.UpdatedAt,
		&userPG.UserType,
	)
	if err != nil {
		return nil, err
	}
	return userPostgresToProto(userPG)

}

func (*server) DeleteUser(ctx context.Context, req *test.DeleteUserRequest) (*test.DeleteUserResponse, error) {
	fmt.Println("Delete user")
	db.QueryRowContext(ctx, deleteUser, req.Id)
	db.QueryRowContext(ctx, deleteItems, req.Id)
	return &test.DeleteUserResponse{}, nil
}

func (*server) ListUser(ctx context.Context, req *test.ListUserRequest) (*test.ListUserResponse, error) {
	fmt.Println("List User")
	res := make([]*test.User, req.GetPageFilter().GetLimit())
	rows, err := db.Query(listUsers, req.GetPageFilter().GetLimit(), req.GetPageFilter().GetPage()*req.GetPageFilter().GetLimit())
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var userPG UserPG
		err = rows.Scan(
			&userPG.Id,
			&userPG.Name,
			&userPG.Age,
			&userPG.CreatedAt,
			&userPG.UpdatedAt,
			&userPG.UserType,
		)
		if err != nil {
			panic(err)
		}
		res[i], _ = userPostgresToProto(userPG)
		i++
	}
	err = rows.Err()
	if err != nil {
		panic(err)
	}
	return &test.ListUserResponse{Users: res}, err
}

func ListItems(userId string) ([]*test.Item, error) {
	fmt.Println("List Items")
	res := make([]*test.Item, 0)
	rows, err := db.Query(listItems, userId)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var userPG ItemPG
		err = rows.Scan(
			&userPG.Id,
			&userPG.Name,
			&userPG.UserId,
			&userPG.CreatedAt,
			&userPG.UpdatedAt,
		)
		if err != nil {
			panic(err)
		}
		proto, err := itemPostgresToProto(userPG)
		if err != nil {
			panic(err)
		}
		res = append(res, proto)
		i++
	}
	err = rows.Err()
	if err != nil {
		panic(err)
	}
	return res, err
}

func (*server) GetUser(ctx context.Context, req *test.GetUserRequest) (*test.User, error) {
	fmt.Println("Get User")
	row := db.QueryRowContext(ctx, getUser, req.Id)
	var i UserPG
	err := row.Scan(
		&i.Id,
		&i.Name,
		&i.Age,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.UserType,
	)
	if err != nil {
		return nil, err
	}
	return userPostgresToProto(i)
}

func (*server) CreateItem(ctx context.Context, req *test.CreateItemRequest) (*test.Item, error) {
	return CreateItem(ctx, req)
}

func CreateItem(ctx context.Context, req *test.CreateItemRequest) (*test.Item, error) {
	fmt.Println("Create Item")

	row := db.QueryRowContext(ctx, addItem, req.Name, req.UserId)
	var i ItemPG
	err := row.Scan(
		&i.Id,
		&i.Name,
		&i.UserId,
		&i.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return itemPostgresToProto(i)
}

func (*server) UpdateItem(ctx context.Context, req *test.UpdateItemRequest) (*test.Item, error) {
	return UpdateItem(ctx, req)
}

func UpdateItem(ctx context.Context, req *test.UpdateItemRequest) (*test.Item, error) {
	fmt.Println("Update Item")
	row := db.QueryRowContext(ctx, updateItem, req.Name, req.Id)
	var i ItemPG
	err := row.Scan(
		&i.Id,
		&i.Name,
		&i.UserId,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return itemPostgresToProto(i)
}

func main() {

	fmt.Println("Start")

	pgURL := os.Getenv("POSTGRES_URL")
	if pgURL == "" {
		panic("POSTGRES_URL must be set")
	}
	parsedURL, err := url.Parse(pgURL)
	if err != nil {
		panic(err)
	}

	db, err = sql.Open("postgres", parsedURL.String())
	if err != nil {
		panic(err)
	}
	err = db.Ping()
	if err != nil {
		panic(err)
	}

	port := defaultPort
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic(err)
	}

	mux := cmux.New(lis)
	grpcL := mux.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
	httpL := mux.Match(cmux.Any())

	go func() {
		sErr := mux.Serve()
		if sErr != nil {
			panic(sErr)
		}
	}()

	s := grpc.NewServer()
	reflection.Register(s)

	test.RegisterServiceExampleServiceServer(s, &server{})

	// Serve gRPC Server
	go func() {
		sErr := s.Serve(grpcL)
		if sErr != nil {
			panic(sErr)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sAddr := fmt.Sprintf("dns:///0.0.0.0:%s", port)
	cc, err := grpc.DialContext(
		ctx,
		sAddr,
		grpc.WithBlock(),
		grpc.WithInsecure(),
	)
	if err != nil {
		panic(err)
	}
	defer cc.Close()

	handler, err := standalone.HandlerViaReflection(ctx, cc, sAddr)
	if err != nil {
		panic(err)
	}

	httpS := &http.Server{
		Handler: handler,
	}

	// Serve HTTP Server
	err = httpS.Serve(httpL)
	if err != http.ErrServerClosed {
		panic(err)
	}
}
