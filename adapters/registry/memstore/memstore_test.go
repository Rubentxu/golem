package memstore

import (
	"context"
	"errors"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

func TestSaveIsAtomicPerCommandID(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()
	rec := ports.CommandReceipt{CommandID: "cmd-1", TenantID: "t", Position: 3}

	if err := r.Save(ctx, rec); err != nil {
		t.Fatal(err)
	}
	err := r.Save(ctx, rec)
	if !errors.Is(err, ports.ErrDuplicateCommand) {
		t.Fatalf("err = %v, want ErrDuplicateCommand", err)
	}

	got, found, err := r.Find(ctx, "cmd-1")
	if err != nil || !found {
		t.Fatalf("find: found=%v err=%v", found, err)
	}
	if got.Position != 3 {
		t.Fatalf("receipt = %+v", got)
	}
	if _, found, _ := r.Find(ctx, "unknown"); found {
		t.Fatal("unknown command reported as found")
	}
}
