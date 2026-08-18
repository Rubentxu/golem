package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// fakePackRegistry drives the HTTP contract tests without the filesystem.
type fakePackRegistry struct {
	loadErr    error
	activateFn func(tenant ports.TenantID, p *ports.LoadedPack) (ports.StreamPosition, error)
	status     *ports.PackStatus
}

func (f *fakePackRegistry) Load(_ context.Context, _ string) (*ports.LoadedPack, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	return &ports.LoadedPack{
		Name:            "demo-pack",
		Version:         "1.0.0",
		IntegrityDigest: strings.Repeat("a", 64),
		RawManifest:     []byte(`{"format_version":"1"}`),
	}, nil
}

func (f *fakePackRegistry) Verify(context.Context, string) error { return nil }

func (f *fakePackRegistry) Activate(ctx context.Context, tenant ports.TenantID, p *ports.LoadedPack) (ports.StreamPosition, error) {
	if f.activateFn != nil {
		return f.activateFn(tenant, p)
	}
	return 7, nil
}

func (f *fakePackRegistry) Status(_ context.Context, _ ports.TenantID, name string) (*ports.PackStatus, error) {
	if f.status != nil {
		return f.status, nil
	}
	return &ports.PackStatus{Name: name, Version: "1.0.0", ActivatedEventID: "evt-1"}, nil
}

func (f *fakePackRegistry) Deactivate(context.Context, ports.TenantID, string) error { return nil }

func activatePackRequest(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/packs/activate", strings.NewReader(body))
	req.Header.Set("X-Golem-Tenant", "tenant-1")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestActivatePack_Accepted(t *testing.T) {
	s := New(nil, nil, nil).WithPacks(&fakePackRegistry{})
	rec := activatePackRequest(t, s, `{"source_path":"demo-pack"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rec.Code, rec.Body)
	}
	var r packActivatedReceipt
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Name != "demo-pack" || r.EventID != "evt-1" || r.Position != 7 {
		t.Errorf("receipt = %+v", r)
	}
}

func TestActivatePack_MissingTenant(t *testing.T) {
	s := New(nil, nil, nil).WithPacks(&fakePackRegistry{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/packs/activate", strings.NewReader(`{"source_path":"x"}`))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestActivatePack_MissingSourcePath(t *testing.T) {
	s := New(nil, nil, nil).WithPacks(&fakePackRegistry{})
	rec := activatePackRequest(t, s, `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestActivatePack_RegistryNotConfigured(t *testing.T) {
	s := New(nil, nil, nil)
	rec := activatePackRequest(t, s, `{"source_path":"x"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestActivatePack_ErrorMapping(t *testing.T) {
	cases := []struct {
		err       error
		wantCode  int
		wantCodeS string
	}{
		{ports.ErrPackNotFound, http.StatusNotFound, CodeNotFound},
		{ports.ErrPackAlreadyActivated, http.StatusConflict, CodeRevisionConflict},
		{ports.ErrPackManifestInvalid, http.StatusUnprocessableEntity, CodeDomainRejection},
		{ports.ErrPackIntegrityFailed, http.StatusUnprocessableEntity, CodeDomainRejection},
		{ports.ErrPackUnknownPermission, http.StatusUnprocessableEntity, CodeDomainRejection},
		{ports.ErrUnsupportedInM51, http.StatusUnprocessableEntity, CodeDomainRejection},
	}
	for _, c := range cases {
		s := New(nil, nil, nil).WithPacks(&fakePackRegistry{loadErr: c.err})
		rec := activatePackRequest(t, s, `{"source_path":"x"}`)
		if rec.Code != c.wantCode {
			t.Errorf("%v: status = %d, want %d; body=%s", c.err, rec.Code, c.wantCode, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), c.wantCodeS) {
			t.Errorf("%v: body missing code %s: %s", c.err, c.wantCodeS, rec.Body)
		}
	}
}
