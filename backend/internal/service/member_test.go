package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestMemberService_UpdateValidation(t *testing.T) {
	s := &MemberService{users: nil, perms: nil, refresh: nil, audit: nil}
	ctx := context.Background()
	id := uuid.New()

	_, err := s.Update(ctx, id, UpdateMemberParams{}, 1, "", "")
	if err != ErrNoMemberFields {
		t.Fatalf("expected ErrNoMemberFields, got %v", err)
	}

	badRole := "admin"
	_, err = s.Update(ctx, id, UpdateMemberParams{Role: &badRole}, 1, "", "")
	if err != ErrInvalidMemberRole {
		t.Fatalf("expected ErrInvalidMemberRole, got %v", err)
	}

	badStatus := "pending"
	_, err = s.Update(ctx, id, UpdateMemberParams{Status: &badStatus}, 1, "", "")
	if err != ErrInvalidMemberStatus {
		t.Fatalf("expected ErrInvalidMemberStatus, got %v", err)
	}
}

func TestMemberService_CreateInvalidRole(t *testing.T) {
	s := &MemberService{}
	_, err := s.Create(context.Background(), CreateMemberInput{
		Email:    "a@b.com",
		Password: "password1",
		Role:     "owner",
	}, 1, "", "")
	if err != ErrInvalidMemberRole {
		t.Fatalf("expected ErrInvalidMemberRole, got %v", err)
	}
}
