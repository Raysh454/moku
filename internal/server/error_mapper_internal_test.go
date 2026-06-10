package server

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/raysh454/moku/internal/registry"
)

func TestMapDomainError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "project not found maps to 404",
			err:        registry.ErrProjectNotFound,
			wantStatus: http.StatusNotFound,
			wantMsg:    registry.ErrProjectNotFound.Error(),
		},
		{
			name:       "website not found maps to 404",
			err:        registry.ErrWebsiteNotFound,
			wantStatus: http.StatusNotFound,
			wantMsg:    registry.ErrWebsiteNotFound.Error(),
		},
		{
			name:       "wrapped not found still maps to 404",
			err:        fmt.Errorf("lookup failed: %w", registry.ErrWebsiteNotFound),
			wantStatus: http.StatusNotFound,
			wantMsg:    registry.ErrWebsiteNotFound.Error(),
		},
		{
			name:       "duplicate slug maps to 409",
			err:        fmt.Errorf("%w: %q", registry.ErrDuplicateSlug, "dup"),
			wantStatus: http.StatusConflict,
			wantMsg:    registry.ErrDuplicateSlug.Error(),
		},
		{
			name:       "arbitrary error maps to generic 500",
			err:        errors.New("UNIQUE constraint failed: websites.slug"),
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "internal server error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, msg := mapDomainError(tc.err)
			if status != tc.wantStatus {
				t.Errorf("status = %d, want %d", status, tc.wantStatus)
			}
			if msg != tc.wantMsg {
				t.Errorf("msg = %q, want %q", msg, tc.wantMsg)
			}
		})
	}
}
