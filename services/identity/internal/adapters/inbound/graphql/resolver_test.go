package graphql

import (
	"context"
	"testing"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/online-shop/pkg/errors"

	"github.com/online-shop/services/identity/internal/adapters/inbound/graphql/model"
	"github.com/online-shop/services/identity/internal/app/ports"
)

type fakeRegistrar struct {
	resp ports.RegisterUserResponse
	err  error
}

func (f fakeRegistrar) Execute(context.Context, ports.RegisterUserRequest) (ports.RegisterUserResponse, error) {
	return f.resp, f.err
}

type fakeAuthenticator struct {
	resp ports.LoginUserResponse
	err  error
}

func (f fakeAuthenticator) Execute(context.Context, ports.LoginUserRequest) (ports.LoginUserResponse, error) {
	return f.resp, f.err
}

type fakeGetter struct {
	resp   ports.GetUserResponse
	err    error
	called bool
}

func (f *fakeGetter) Execute(context.Context, ports.GetUserRequest) (ports.GetUserResponse, error) {
	f.called = true
	return f.resp, f.err
}

func TestToGraphQLError_MapsKindToCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"invalid", errors.NewInvalid("c", "m", nil), "INVALID"},
		{"not found", errors.NewNotFound("c", "m", nil), "NOT_FOUND"},
		{"conflict", errors.NewConflict("c", "m", nil), "CONFLICT"},
		{"unauthorized", errors.NewUnauthorized("c", "m", nil), "UNAUTHORIZED"},
		{"forbidden", errors.NewForbidden("c", "m", nil), "FORBIDDEN"},
		{"internal", errors.NewInternal("c", "m", nil), "INTERNAL"},
		{"plain error", context.DeadlineExceeded, "INTERNAL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := asGQLError(t, toGraphQLError(tc.err)).Extensions["code"]; got != tc.want {
				t.Fatalf("code = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestToGraphQLError_InternalMessageIsGeneric(t *testing.T) {
	t.Parallel()

	gErr := asGQLError(t, toGraphQLError(errors.NewInternal("identity.secret", "connection string leaked", nil)))
	if gErr.Message != "internal error" {
		t.Fatalf("internal message must be generic, got %q", gErr.Message)
	}
}

func TestRegisterUser_ReadsBackFullEntity(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	getter := &fakeGetter{resp: ports.GetUserResponse{User: ports.UserView{
		ID: "u-1", Email: "a@b.c", Roles: []string{"customer"}, CreatedAt: created,
	}}}
	r := NewResolver(fakeRegistrar{resp: ports.RegisterUserResponse{UserID: "u-1", Email: "a@b.c"}}, nil, getter)

	user, err := r.Mutation().RegisterUser(context.Background(), model.RegisterUserInput{Email: "a@b.c", Password: "hunter2!"})
	if err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}
	if !getter.called {
		t.Fatal("RegisterUser must read the user back to return a full entity")
	}
	if user.ID != "u-1" || user.Email != "a@b.c" || len(user.Roles) != 1 || !user.CreatedAt.Equal(created) {
		t.Fatalf("user = %+v", user)
	}
}

func TestLoginUser_MapsAuthPayload(t *testing.T) {
	t.Parallel()

	exp := time.Date(2026, 1, 1, 0, 15, 0, 0, time.UTC)
	r := NewResolver(nil, fakeAuthenticator{resp: ports.LoginUserResponse{
		AccessToken: "a.jwt", RefreshToken: "refresh", AccessTokenExpiresAt: exp,
	}}, nil)

	payload, err := r.Mutation().LoginUser(context.Background(), model.LoginUserInput{Email: "a@b.c", Password: "hunter2!"})
	if err != nil {
		t.Fatalf("LoginUser: %v", err)
	}
	if payload.AccessToken != "a.jwt" || payload.RefreshToken != "refresh" || !payload.AccessTokenExpiresAt.Equal(exp) {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestMe_RequiresCaller(t *testing.T) {
	t.Parallel()

	view := ports.GetUserResponse{User: ports.UserView{ID: "u-1", Email: "a@b.c"}}

	t.Run("anonymous is unauthorized and never hits the use case", func(t *testing.T) {
		t.Parallel()
		getter := &fakeGetter{resp: view}
		_, err := NewResolver(nil, nil, getter).Query().Me(context.Background())
		if got := asGQLError(t, err).Extensions["code"]; got != "UNAUTHORIZED" {
			t.Fatalf("code = %v, want UNAUTHORIZED", got)
		}
		if getter.called {
			t.Fatal("use case must not run without a caller")
		}
	})

	t.Run("caller reads its own user", func(t *testing.T) {
		t.Parallel()
		getter := &fakeGetter{resp: view}
		ctx := withCaller(context.Background(), caller{userID: "u-1"})
		user, err := NewResolver(nil, nil, getter).Query().Me(ctx)
		if err != nil {
			t.Fatalf("Me: %v", err)
		}
		if !getter.called || user.ID != "u-1" {
			t.Fatalf("user = %+v called = %v", user, getter.called)
		}
	})
}

func asGQLError(t *testing.T, err error) *gqlerror.Error {
	t.Helper()
	g, ok := err.(*gqlerror.Error)
	if !ok {
		t.Fatalf("error is %T, want *gqlerror.Error", err)
	}
	return g
}
