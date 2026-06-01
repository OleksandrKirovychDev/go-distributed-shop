package grpc

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/online-shop/pkg/errors"

	identityv1 "github.com/online-shop/proto/gen/go/identity/v1"

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

type fakeRefresher struct {
	resp ports.RefreshTokenResponse
	err  error
}

func (f fakeRefresher) Execute(context.Context, ports.RefreshTokenRequest) (ports.RefreshTokenResponse, error) {
	return f.resp, f.err
}

type fakeVerifier struct {
	resp ports.VerifyTokenResponse
	err  error
}

func (f fakeVerifier) Execute(context.Context, ports.VerifyTokenRequest) (ports.VerifyTokenResponse, error) {
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

func TestToStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"invalid", errors.NewInvalid("c", "m", nil), codes.InvalidArgument},
		{"not found", errors.NewNotFound("c", "m", nil), codes.NotFound},
		{"conflict", errors.NewConflict("c", "m", nil), codes.AlreadyExists},
		{"unauthorized", errors.NewUnauthorized("c", "m", nil), codes.Unauthenticated},
		{"forbidden", errors.NewForbidden("c", "m", nil), codes.PermissionDenied},
		{"internal", errors.NewInternal("c", "m", nil), codes.Internal},
		{"plain error", context.DeadlineExceeded, codes.Internal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := status.Code(toStatus(tc.err)); got != tc.want {
				t.Fatalf("toStatus(%v) code = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestInternalErrorMessageIsGeneric(t *testing.T) {
	t.Parallel()

	st := status.Convert(toStatus(errors.NewInternal("identity.secret", "connection string leaked", nil)))
	if st.Message() != "internal error" {
		t.Fatalf("internal error message must be generic, got %q", st.Message())
	}
}

func TestRegisterUser_MapsResponse(t *testing.T) {
	t.Parallel()

	s := NewServer(fakeRegistrar{resp: ports.RegisterUserResponse{UserID: "u-1", Email: "a@b.c"}}, nil, nil, nil, nil)

	resp, err := s.RegisterUser(context.Background(), &identityv1.RegisterUserRequest{Email: "a@b.c", Password: "hunter2!"})
	if err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}
	if resp.GetUserId() != "u-1" || resp.GetEmail() != "a@b.c" {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestRegisterUser_MapsError(t *testing.T) {
	t.Parallel()

	s := NewServer(fakeRegistrar{err: errors.NewConflict("identity.email_taken", "email already registered", nil)}, nil, nil, nil, nil)

	_, err := s.RegisterUser(context.Background(), &identityv1.RegisterUserRequest{Email: "a@b.c", Password: "hunter2!"})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("code = %v, want AlreadyExists", status.Code(err))
	}
}

func TestLoginUser_MapsTimestamps(t *testing.T) {
	t.Parallel()

	exp := time.Date(2026, 1, 1, 0, 15, 0, 0, time.UTC)
	s := NewServer(nil, fakeAuthenticator{resp: ports.LoginUserResponse{
		AccessToken: "a.jwt", RefreshToken: "refresh", AccessTokenExpiresAt: exp,
	}}, nil, nil, nil)

	resp, err := s.LoginUser(context.Background(), &identityv1.LoginUserRequest{Email: "a@b.c", Password: "hunter2!"})
	if err != nil {
		t.Fatalf("LoginUser: %v", err)
	}
	if resp.GetAccessToken() != "a.jwt" || resp.GetRefreshToken() != "refresh" {
		t.Fatalf("resp = %+v", resp)
	}
	if !resp.GetAccessTokenExpiresAt().AsTime().Equal(exp) {
		t.Fatalf("expiry = %v, want %v", resp.GetAccessTokenExpiresAt().AsTime(), exp)
	}
}

func TestRefreshToken_MapsResponse(t *testing.T) {
	t.Parallel()

	exp := time.Date(2026, 1, 1, 0, 15, 0, 0, time.UTC)
	s := NewServer(nil, nil, fakeRefresher{resp: ports.RefreshTokenResponse{
		AccessToken: "new.jwt", RefreshToken: "rotated", AccessTokenExpiresAt: exp,
	}}, nil, nil)

	resp, err := s.RefreshToken(context.Background(), &identityv1.RefreshTokenRequest{RefreshToken: "old"})
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if resp.GetAccessToken() != "new.jwt" || resp.GetRefreshToken() != "rotated" {
		t.Fatalf("resp = %+v", resp)
	}
	if !resp.GetAccessTokenExpiresAt().AsTime().Equal(exp) {
		t.Fatalf("expiry = %v, want %v", resp.GetAccessTokenExpiresAt().AsTime(), exp)
	}
}

func TestVerifyToken_MapsResponse(t *testing.T) {
	t.Parallel()

	s := NewServer(nil, nil, nil, fakeVerifier{resp: ports.VerifyTokenResponse{
		Valid: true, UserID: "u-1", Email: "a@b.c", Roles: []string{"customer"},
	}}, nil)

	resp, err := s.VerifyToken(context.Background(), &identityv1.VerifyTokenRequest{AccessToken: "tok"})
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if !resp.GetValid() || resp.GetUserId() != "u-1" || resp.GetEmail() != "a@b.c" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(resp.GetRoles()) != 1 || resp.GetRoles()[0] != "customer" {
		t.Fatalf("roles = %v", resp.GetRoles())
	}
}

func TestGetUser_Authorization(t *testing.T) {
	t.Parallel()

	view := ports.GetUserResponse{User: ports.UserView{ID: "u-1", Email: "a@b.c", Roles: []string{"customer"}, CreatedAt: time.Now()}}

	cases := []struct {
		name       string
		md         metadata.MD
		wantCode   codes.Code
		wantCalled bool
	}{
		{"anonymous denied", nil, codes.PermissionDenied, false},
		{"other user denied", metadata.Pairs(mdUserID, "u-2"), codes.PermissionDenied, false},
		{"self allowed", metadata.Pairs(mdUserID, "u-1"), codes.OK, true},
		{"admin allowed", metadata.Pairs(mdUserRoles, "admin"), codes.OK, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			getter := &fakeGetter{resp: view}
			s := NewServer(nil, nil, nil, nil, getter)

			ctx := context.Background()
			if tc.md != nil {
				ctx = metadata.NewIncomingContext(ctx, tc.md)
			}

			_, err := s.GetUser(ctx, &identityv1.GetUserRequest{UserId: "u-1"})
			if status.Code(err) != tc.wantCode {
				t.Fatalf("code = %v, want %v", status.Code(err), tc.wantCode)
			}
			if getter.called != tc.wantCalled {
				t.Fatalf("use case called = %v, want %v", getter.called, tc.wantCalled)
			}
		})
	}
}
